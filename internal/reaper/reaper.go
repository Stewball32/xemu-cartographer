// Package reaper reclaims idle player-hosted xemu instances. A box provisioned
// via POST /api/play/request that nobody joins — no live match, no second
// System Link machine — for the configured idle window is torn down so
// abandoned boxes don't pile up during a friends test. Being played (phase
// live) or joined (a guest machine is connected) resets the idle timer, so an
// active box is never reaped.
//
// The idle-tracking state machine here is pure and clock-injected (fully
// unit-tested). The signal source (per-instance activity) and the teardown
// (stop scraper runner + remove container) are injected as small interfaces so
// this package has no compile-time dependency on the scraper manager or podman
// — main.go supplies the concrete adapters, mirroring the play routes' DI.
//
// Only instances whose name carries the configured prefix (default "play-",
// the per-user boxes request-instance derives) are ever considered, so
// admin/manual containers are never auto-reaped.
package reaper

import (
	"context"
	"strings"
	"sync"
	"time"
)

// Snapshot is one instance's liveness as the reaper reads it each pass.
type Snapshot struct {
	Instance string
	// Active is true when the box is being played (a live match) or has other
	// machines joined — either resets its idle timer. Only a box that is up but
	// unused (inactive) accrues idle time toward reaping.
	Active bool
}

// Source yields the current live instances + their activity each pass. The
// adapter builds it from the scraper's per-instance phase + joined-machine
// count.
type Source interface {
	Snapshot() []Snapshot
}

// Remover tears one instance down: stop its scraper runner + remove its
// container. Called by the reaper when a box exceeds the idle window.
type Remover interface {
	Reap(instance string) error
}

// Config controls the reaper. A zero (or negative) IdleTimeout disables
// reaping entirely — a fail-safe so a misconfigured/zeroed value never
// aggressively tears boxes down.
type Config struct {
	IdleTimeout time.Duration // reap after a box has been idle this long
	WarnBefore  time.Duration // heads-up window before reap (surfaced to the play UI)
	Poll        time.Duration // pass interval (Run loop)
	NamePrefix  string        // only consider instances whose name has this prefix ("" = all)
}

// Info is the per-instance countdown the play API surfaces to the host so the
// UI can warn "your idle box will be released in Nm" before the reaper acts.
// Idle is false (zero value) when the instance isn't currently accruing idle
// time (active, unknown, or reaping disabled).
type Info struct {
	Idle      bool      `json:"idle"`
	IdleSince time.Time `json:"idle_since"`
	ReapAt    time.Time `json:"reap_at"`
	Warning   bool      `json:"warning"` // now within WarnBefore of ReapAt
}

// Reaper tracks how long each idle instance has been idle and tears down those
// that exceed the window. Safe for concurrent use: Tick runs on the loop
// goroutine while Info is read from request goroutines (the play API).
type Reaper struct {
	cfg Config
	src Source
	rem Remover
	now func() time.Time
	log func(format string, args ...any)

	mu        sync.Mutex
	idleSince map[string]time.Time // instance -> first idle observation
}

// Option customises a Reaper at construction (clock + logger injection for
// tests). Both default to sensible production values when unset.
type Option func(*Reaper)

// WithClock overrides the time source (tests inject a controllable clock).
func WithClock(now func() time.Time) Option {
	return func(r *Reaper) { r.now = now }
}

// WithLogger overrides the log sink (defaults to a no-op; main.go wires log.Printf).
func WithLogger(log func(format string, args ...any)) Option {
	return func(r *Reaper) { r.log = log }
}

// New builds a Reaper. src/rem may be nil only in tests that never call Tick.
func New(cfg Config, src Source, rem Remover, opts ...Option) *Reaper {
	r := &Reaper{
		cfg:       cfg,
		src:       src,
		rem:       rem,
		now:       time.Now,
		log:       func(string, ...any) {},
		idleSince: map[string]time.Time{},
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// Enabled reports whether reaping is active (a positive idle timeout).
func (r *Reaper) Enabled() bool { return r.cfg.IdleTimeout > 0 }

// Tick runs one reap pass: refresh each tracked instance's idle clock from the
// current snapshot, then reap any that have exceeded the idle window. Exported
// so tests can drive passes deterministically without the Run loop.
func (r *Reaper) Tick() {
	if !r.Enabled() {
		return
	}
	snaps := r.src.Snapshot()
	now := r.now()

	present := make(map[string]struct{}, len(snaps))
	var toReap []string

	r.mu.Lock()
	for _, s := range snaps {
		present[s.Instance] = struct{}{}
		if !r.matches(s.Instance) {
			continue
		}
		if s.Active {
			// Being played / joined — reset the idle clock.
			delete(r.idleSince, s.Instance)
			continue
		}
		since, ok := r.idleSince[s.Instance]
		if !ok {
			// First idle observation — start the clock, don't reap yet.
			r.idleSince[s.Instance] = now
			continue
		}
		if now.Sub(since) >= r.cfg.IdleTimeout {
			toReap = append(toReap, s.Instance)
			delete(r.idleSince, s.Instance)
		}
	}
	// Prune tracking for instances that vanished (removed elsewhere / stopped)
	// so a name reused later starts its clock fresh.
	for inst := range r.idleSince {
		if _, ok := present[inst]; !ok {
			delete(r.idleSince, inst)
		}
	}
	r.mu.Unlock()

	// Reap outside the lock — Remover.Reap shells out to podman and may block.
	for _, inst := range toReap {
		if err := r.rem.Reap(inst); err != nil {
			r.log("reaper: reap %s: %v", inst, err)
			continue
		}
		r.log("reaper: reaped idle instance %s (idle >= %s)", inst, r.cfg.IdleTimeout)
	}
}

// Info returns the caller-facing idle countdown for one instance. ok is folded
// into Info.Idle: a not-currently-idle instance returns the zero Info.
func (r *Reaper) Info(instance string) Info {
	if !r.Enabled() {
		return Info{}
	}
	r.mu.Lock()
	since, ok := r.idleSince[instance]
	r.mu.Unlock()
	if !ok {
		return Info{}
	}
	reapAt := since.Add(r.cfg.IdleTimeout)
	warn := r.cfg.WarnBefore > 0 && !r.now().Before(reapAt.Add(-r.cfg.WarnBefore))
	return Info{Idle: true, IdleSince: since, ReapAt: reapAt, Warning: warn}
}

// Run drives Tick on a ticker until ctx is cancelled. No-op (returns
// immediately) when reaping is disabled, so main.go can start it
// unconditionally.
func (r *Reaper) Run(ctx context.Context) {
	if !r.Enabled() {
		return
	}
	poll := r.cfg.Poll
	if poll <= 0 {
		poll = 30 * time.Second
	}
	t := time.NewTicker(poll)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.Tick()
		}
	}
}

// matches reports whether an instance name is in the reaper's scope.
func (r *Reaper) matches(instance string) bool {
	if r.cfg.NamePrefix == "" {
		return true
	}
	return strings.HasPrefix(instance, r.cfg.NamePrefix)
}
