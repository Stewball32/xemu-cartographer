package hostrunner

import (
	"fmt"
	"sync/atomic"
	"time"
)

// Input is the write side — satisfied directly by *vncinput.Injector (Tap /
// Chord). Faked in tests. Keeping it an interface is what lets all runner logic
// be unit-tested with no live container.
type Input interface {
	Tap(label string) error
	Chord(labels ...string) error
}

type nopInput struct{}

func (nopInput) Tap(string) error      { return nil }
func (nopInput) Chord(...string) error { return nil }

// Config configures one instance's runner.
type Config struct {
	Instance string
	Timing   StepTiming
	Selector Selector
	Start    StartPolicy

	// ReadyFn is the optional ready-gate (requirement 6, default OFF = nil).
	// When set it's appended as a StartPredicate so start also waits on it.
	ReadyFn func(Observation) bool
}

// Runner is the state-aware auto-host runner for one instance. It reads
// Observations (built by the integration layer from the scraper's cache — never
// by touching a GameReader from a new goroutine) and drives the host lobby via
// Input, gating every press on readable state. Not safe for concurrent Tick
// calls — driven from one goroutine (the scraper loop); only the Arbiter is
// concurrency-safe for control endpoints.
type Runner struct {
	cfg   Config
	seq   *Sequence
	arb   *Arbiter
	input Input
	sink  EventSink

	armed     bool
	started   bool
	startAt   time.Time
	lastPhase Phase

	// lastObs is the most recent observation, stored so execute() can enforce the
	// SAFETY interlock (never send B on SELECT MAP — it ends the live lobby) at the
	// single key-emission choke point, regardless of which path produced the action.
	lastObs Observation

	// Re-select state: lobbyApplied flips true once the sequence first reaches the
	// created lobby; appliedGen is the selector generation that lobby reflects. A
	// later pick (Generation() != appliedGen) triggers a re-select to CHANGE the live
	// lobby's map/gametype (ReselectSequence).
	lobbyApplied bool
	appliedGen   uint64

	// ready is the player-scoped start request (requirement 6 made live): the
	// /api/play/ready route flips it so the runner presses start once the native
	// preconditions pass, even under the default arm-only policy. Atomic because
	// a request goroutine writes it while the tick goroutine reads it.
	ready atomic.Bool
}

// New builds a Runner. nil input/sink default to no-ops; empty config fields get
// sensible v1 defaults (default selector, arm-only native-ready start policy).
func New(cfg Config, input Input, sink EventSink) *Runner {
	if cfg.Timing == (StepTiming{}) {
		cfg.Timing = DefaultTiming
	}
	if cfg.Selector == nil {
		cfg.Selector = DefaultSelector()
	}
	if len(cfg.Start.Predicates) == 0 {
		cfg.Start = DefaultStartPolicy()
	}
	if cfg.ReadyFn != nil {
		cfg.Start.Predicates = append(cfg.Start.Predicates, ReadyGatePredicate(cfg.ReadyFn))
	}
	if input == nil {
		input = nopInput{}
	}
	if sink == nil {
		sink = NopSink{}
	}
	return &Runner{
		cfg:   cfg,
		seq:   DefaultHostSequence(cfg.Timing, cfg.Selector),
		arb:   NewArbiter(),
		input: input,
		sink:  sink,
	}
}

// Arbiter exposes the arbitration control (for the admin endpoints).
func (r *Runner) Arbiter() *Arbiter { return r.arb }

// Sequence exposes the lobby sequence (for tests / inspection).
func (r *Runner) Sequence() *Sequence { return r.seq }

// SetReady sets the player-scoped start request. true = press start once native
// conditions pass (arm+start); false = stay armed in the lobby (arm-only).
func (r *Runner) SetReady(v bool) { r.ready.Store(v) }

// Ready reports the current player-scoped start request.
func (r *Runner) Ready() bool { return r.ready.Load() }

// SetSelection records the player's map / gametype picks (with the D-pad Steps
// the runner navigates to each chosen card) when the runner's Selector is an
// *AtomicSelector. Setting a selection un-parks the runner: it applies the pick
// in one forward pass. Returns false when the runner was built with a non-mutable
// selector.
func (r *Runner) SetSelection(mapPick, gametypePick Pick) bool {
	sel, ok := r.cfg.Selector.(*AtomicSelector)
	if !ok {
		return false
	}
	sel.Set(mapPick, gametypePick)
	return true
}

// ClearSelection resets the runner to unselected so it re-parks at map-select
// awaiting the next pick (e.g. a player teardown). Returns false for a non-mutable
// selector.
func (r *Runner) ClearSelection() bool {
	sel, ok := r.cfg.Selector.(*AtomicSelector)
	if !ok {
		return false
	}
	sel.Clear()
	return true
}

// HasSelection reports whether a map/gametype has been chosen (the park gate).
func (r *Runner) HasSelection() bool { return r.cfg.Selector.HasSelection() }

// Selection returns the runner's current map / gametype picks (intent).
func (r *Runner) Selection() (mapPick, gametypePick Pick) {
	return r.cfg.Selector.MapPick(), r.cfg.Selector.GametypePick()
}

// Tick decides + executes + broadcasts one step. It stands down (no input) when
// an admin holds authority or the runner is disabled, and never acts on a stale
// observation. Returns the Action it took for the caller / tests.
func (r *Runner) Tick(obs Observation, now time.Time) Action {
	defer func() { r.lastPhase = obs.Phase }()
	r.lastObs = obs // for the execute() SAFETY interlock

	if !r.arb.CanEmit() {
		act := wait("runner suspended (" + r.arb.Authority().String() + ")")
		r.emit(obs, "suspended", act, nil)
		return act
	}
	if !obs.Fresh {
		act := wait("no fresh read")
		r.emit(obs, act.Kind.String(), act, nil)
		return act
	}

	act := guardDestructiveBack(r.decide(obs, now), obs) // SAFETY: never B on SELECT MAP
	err := r.execute(act)
	r.emit(obs, act.Kind.String(), act, err)
	return act
}

// decide is the pure decision (no I/O) — the auto-host loop + gated sequence.
// Split out so tests can assert decisions without a fake Input.
//
// The runner's job ENDS at a created System Link lobby with the player's map +
// gametype selected. It does NOT start the match: the PLAYERS press start
// themselves. There is no "ready" gate and no native 2-player/2-team start gate —
// both were removed (Stewart 2026-08); the runner never presses A to start.
func (r *Runner) decide(obs Observation, now time.Time) Action {
	switch Classify(obs) {
	case ScreenInGame:
		r.armed = false
		return wait("match live")
	case ScreenPostGame:
		// Clear the post-game / carnage screen so a fresh lobby can be set up.
		r.armed = false
		return tap("a", "clear carnage", "post-game screen — tapping A to clear")
	}

	// Just returned from a game to a hostable menu → re-host from the top. Rebuild the
	// DEFAULT sequence (r.seq may currently be a ReselectSequence) and clear the
	// re-select bookkeeping so the next lobby starts fresh.
	if (r.lastPhase == PhaseInGame || r.lastPhase == PhasePostGame) &&
		(obs.Phase == PhaseMenu || obs.Phase == PhasePreGame) {
		r.seq = DefaultHostSequence(r.cfg.Timing, r.cfg.Selector)
		r.seq.Reset(now)
		r.armed = false
		r.lobbyApplied = false
		r.appliedGen = 0
	}

	act := r.seq.Step(obs, now)
	if act.Kind != ActionDone {
		return act
	}

	// DONE — lobby created, map + gametype selected. The runner stops here and just
	// observes (it keeps ticking + emitting state so /play still reflects the live
	// lobby). Players start the match.
	r.armed = true

	// RE-SELECT: if the player picked a NEW map/gametype after this lobby was created,
	// CHANGE the live lobby's selection — SAFELY: back out to SELECT GAMETYPE then
	// SELECT MAP (both safe), re-drive forward, return. Never B on SELECT MAP (the
	// interlock). Detected via the selector's monotonic generation.
	if sel, ok := r.cfg.Selector.(*AtomicSelector); ok {
		gen := sel.Generation()
		switch {
		case !r.lobbyApplied:
			// First created lobby — record the generation it reflects; do NOT re-select.
			r.lobbyApplied = true
			r.appliedGen = gen
		case gen != r.appliedGen:
			// A newer pick than this lobby reflects → drive the re-select from here.
			r.appliedGen = gen
			r.seq = ReselectSequence(r.cfg.Timing, r.cfg.Selector)
			r.seq.Reset(now)
			return r.seq.Step(obs, now)
		}
	}
	return wait("lobby ready — map + gametype selected; players start the match")
}

// WalkBack is an operator/admin command: press B to back out one screen, or
// cancel a running countdown. Gated only on a fresh read.
func (r *Runner) WalkBack(obs Observation, now time.Time) Action {
	r.lastObs = obs // for the execute() SAFETY interlock
	if !r.arb.CanEmit() {
		act := wait("runner suspended (" + r.arb.Authority().String() + ")")
		r.emit(obs, "suspended", act, nil)
		return act
	}
	act := guardDestructiveBack(r.seq.WalkBack(obs, now), obs) // SAFETY: never B on SELECT MAP
	r.armed, r.started = false, false
	err := r.execute(act)
	r.emit(obs, act.Kind.String(), act, err)
	return act
}

// GatedPress is the FIRST LIVE MILESTONE: read state, confirm we're on `expect`,
// then tap `key` exactly once via Input — the minimal read→act→verify proof that
// vncinput drives a live container, before the full sequence is trusted. Blocks
// (no press) if the observation says we're on a different screen.
func (r *Runner) GatedPress(obs Observation, expect Screen, key string) Action {
	act := GatedPress(obs, expect, key)
	if !r.arb.CanEmit() {
		act = wait("runner suspended (" + r.arb.Authority().String() + ")")
		r.emit(obs, "suspended", act, nil)
		return act
	}
	var err error
	if act.Kind == ActionTap {
		err = r.execute(act)
	}
	r.emit(obs, act.Kind.String(), act, err)
	return act
}

// GatedPress (pure) is the decision half of the first-live-milestone: tap only
// when the observation confirms `expect`, else Blocked/Wait — never blind.
func GatedPress(obs Observation, expect Screen, key string) Action {
	if !obs.Fresh {
		return wait("no fresh read")
	}
	got := Classify(obs)
	if got != expect {
		return blocked(fmt.Sprintf("expected %s, observed %s — not pressing", expect, got))
	}
	return tap(key, "gated single press", "confirmed "+expect.String()+", pressing "+key)
}

func (r *Runner) execute(act Action) error {
	switch act.Kind {
	case ActionTap:
		k := act.Key()
		// SAFETY INTERLOCK (hard backstop, single key-emission choke point): B on the
		// SELECT MAP screen ENDS a live lobby and disconnects ALL joined players (the
		// lobby is created the moment SELECT MAP is reached). Refuse to send it here no
		// matter which path produced it — unknown/off-route recovery, stuck-detection,
		// WalkBack, a mis-timed re-select back-out. The map list being up
		// (MapCursorCount>0) means we're on SELECT MAP. The only legitimate B's are on
		// the pregame lobby and SELECT GAMETYPE (map list down); those pass.
		if k == "b" && r.lastObs.MapCursorCount > 0 {
			return nil // never sent — the game must not be killed by a stray Back
		}
		if k != "" {
			return r.input.Tap(k)
		}
	case ActionChord:
		if len(act.Keys) > 0 {
			return r.input.Chord(act.Keys...)
		}
	}
	return nil
}

// guardDestructiveBack converts a would-be B on SELECT MAP into a visible blocked
// Action, so the decision log/stream shows the refusal (the execute() interlock is
// the hard backstop that also silently drops it). SELECT MAP = the map list is up.
func guardDestructiveBack(act Action, obs Observation) Action {
	if act.Kind == ActionTap && act.Key() == "b" && obs.MapCursorCount > 0 {
		return blocked("SAFETY: refusing B on SELECT MAP — it would end the live lobby and drop all joined players")
	}
	return act
}

func (r *Runner) emit(obs Observation, kind string, act Action, err error) {
	ev := buildEvent(r.cfg.Instance, obs, r.arb.Authority(), kind, act)
	if err != nil {
		if ev.Reason != "" {
			ev.Reason += "; "
		}
		ev.Reason += "input error: " + err.Error()
	}
	r.sink.Emit(ev)
}
