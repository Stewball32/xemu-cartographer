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

	act := r.decide(obs, now)
	err := r.execute(act)
	r.emit(obs, act.Kind.String(), act, err)
	return act
}

// decide is the pure decision (no I/O) — the auto-host loop + gated sequence +
// arm/start. Split out so tests can assert decisions without a fake Input.
func (r *Runner) decide(obs Observation, now time.Time) Action {
	switch Classify(obs) {
	case ScreenInGame:
		r.armed, r.started = false, false
		return wait("match live")
	case ScreenPostGame:
		// Auto-host loop: clear the post-game / carnage screen so we can re-host.
		r.armed, r.started = false, false
		return tap("a", "clear carnage", "post-game screen — tapping A to clear")
	}

	// Just returned from a game to a hostable menu → re-host from the top.
	if (r.lastPhase == PhaseInGame || r.lastPhase == PhasePostGame) &&
		(obs.Phase == PhaseMenu || obs.Phase == PhasePreGame) {
		r.seq.Reset(now)
		r.armed, r.started = false, false
	}

	act := r.seq.Step(obs, now)
	if act.Kind != ActionDone {
		return act
	}

	// Sequence complete = settled in the lobby with the gametype selected (armed).
	r.armed = true
	// arm+start when the policy says so OR a player has hit "ready" in the play
	// tab — the native predicates below still gate the actual start press.
	if r.cfg.Start.Mode != ArmAndStart && !r.ready.Load() {
		return wait("armed; arm-only — awaiting operator/admin/player start")
	}
	if ok, why := r.cfg.Start.Evaluate(obs); !ok {
		return wait("armed; holding start: " + why)
	}
	if obs.CountdownActive {
		return wait("countdown running")
	}
	if r.started && now.Sub(r.startAt) < r.cfg.Timing.RepressAfter {
		return wait("start pressed; awaiting countdown")
	}
	r.started, r.startAt = true, now
	return tap("a", "start countdown", "native ready + arm+start — tapping A to start")
}

// WalkBack is an operator/admin command: press B to back out one screen, or
// cancel a running countdown. Gated only on a fresh read.
func (r *Runner) WalkBack(obs Observation, now time.Time) Action {
	if !r.arb.CanEmit() {
		act := wait("runner suspended (" + r.arb.Authority().String() + ")")
		r.emit(obs, "suspended", act, nil)
		return act
	}
	act := r.seq.WalkBack(obs, now)
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
		if k := act.Key(); k != "" {
			return r.input.Tap(k)
		}
	case ActionChord:
		if len(act.Keys) > 0 {
			return r.input.Chord(act.Keys...)
		}
	}
	return nil
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
