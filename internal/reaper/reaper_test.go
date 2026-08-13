package reaper

import (
	"sync"
	"testing"
	"time"
)

// fakeSource returns a scripted snapshot; mutate snaps between Tick calls.
type fakeSource struct {
	mu    sync.Mutex
	snaps []Snapshot
}

func (f *fakeSource) set(s ...Snapshot) {
	f.mu.Lock()
	f.snaps = s
	f.mu.Unlock()
}

func (f *fakeSource) Snapshot() []Snapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Snapshot, len(f.snaps))
	copy(out, f.snaps)
	return out
}

// fakeRemover records every reap call.
type fakeRemover struct {
	mu      sync.Mutex
	reaped  []string
	failFor map[string]error
}

func (f *fakeRemover) Reap(instance string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.failFor[instance]; err != nil {
		return err
	}
	f.reaped = append(f.reaped, instance)
	return nil
}

func (f *fakeRemover) reapedList() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.reaped))
	copy(out, f.reaped)
	return out
}

// clock is a controllable time source.
type clock struct{ t time.Time }

func (c *clock) now() time.Time      { return c.t }
func (c *clock) add(d time.Duration) { c.t = c.t.Add(d) }

func newTestReaper(cfg Config, src Source, rem Remover, c *clock) *Reaper {
	return New(cfg, src, rem, WithClock(c.now))
}

func TestReapsIdleInstanceAfterTimeout(t *testing.T) {
	src := &fakeSource{}
	rem := &fakeRemover{}
	c := &clock{t: time.Unix(1_000_000, 0)}
	r := newTestReaper(Config{IdleTimeout: 10 * time.Minute, NamePrefix: "play-"}, src, rem, c)

	src.set(Snapshot{Instance: "play-a", Active: false})

	// First pass: start the idle clock, do not reap.
	r.Tick()
	if got := rem.reapedList(); len(got) != 0 {
		t.Fatalf("first pass should not reap, got %v", got)
	}
	info := r.Info("play-a")
	if !info.Idle {
		t.Fatalf("play-a should be tracked idle after first pass")
	}

	// Still under the window.
	c.add(9 * time.Minute)
	r.Tick()
	if got := rem.reapedList(); len(got) != 0 {
		t.Fatalf("under window should not reap, got %v", got)
	}

	// Crosses the window.
	c.add(2 * time.Minute)
	r.Tick()
	if got := rem.reapedList(); len(got) != 1 || got[0] != "play-a" {
		t.Fatalf("expected reap of play-a, got %v", got)
	}
	// After reaping, tracking is cleared.
	if r.Info("play-a").Idle {
		t.Fatalf("play-a should no longer be tracked after reap")
	}
}

func TestActiveInstanceNeverReaped(t *testing.T) {
	src := &fakeSource{}
	rem := &fakeRemover{}
	c := &clock{t: time.Unix(1_000_000, 0)}
	r := newTestReaper(Config{IdleTimeout: 1 * time.Minute, NamePrefix: "play-"}, src, rem, c)

	src.set(Snapshot{Instance: "play-live", Active: true})
	for i := 0; i < 10; i++ {
		c.add(5 * time.Minute)
		r.Tick()
	}
	if got := rem.reapedList(); len(got) != 0 {
		t.Fatalf("active instance must never be reaped, got %v", got)
	}
	if r.Info("play-live").Idle {
		t.Fatalf("active instance must not be tracked idle")
	}
}

func TestActivityResetsIdleClock(t *testing.T) {
	src := &fakeSource{}
	rem := &fakeRemover{}
	c := &clock{t: time.Unix(1_000_000, 0)}
	r := newTestReaper(Config{IdleTimeout: 10 * time.Minute, NamePrefix: "play-"}, src, rem, c)

	// Idle for a while (but under the window).
	src.set(Snapshot{Instance: "play-x", Active: false})
	r.Tick()
	c.add(8 * time.Minute)
	r.Tick()

	// A guest joins → active → clock resets.
	src.set(Snapshot{Instance: "play-x", Active: true})
	c.add(1 * time.Minute)
	r.Tick()
	if r.Info("play-x").Idle {
		t.Fatalf("activity should have reset the idle clock")
	}

	// Goes idle again — the full window must elapse from HERE.
	src.set(Snapshot{Instance: "play-x", Active: false})
	c.add(1 * time.Minute)
	r.Tick() // restart clock
	c.add(9 * time.Minute)
	r.Tick()
	if got := rem.reapedList(); len(got) != 0 {
		t.Fatalf("clock should have restarted after activity, got %v", got)
	}
	c.add(2 * time.Minute)
	r.Tick()
	if got := rem.reapedList(); len(got) != 1 || got[0] != "play-x" {
		t.Fatalf("expected reap after fresh window, got %v", got)
	}
}

func TestPrefixFilteringSkipsNonPlayBoxes(t *testing.T) {
	src := &fakeSource{}
	rem := &fakeRemover{}
	c := &clock{t: time.Unix(1_000_000, 0)}
	r := newTestReaper(Config{IdleTimeout: 1 * time.Minute, NamePrefix: "play-"}, src, rem, c)

	src.set(
		Snapshot{Instance: "smoke", Active: false},  // admin/manual box — out of scope
		Snapshot{Instance: "play-1", Active: false}, // in scope
	)
	r.Tick()
	c.add(5 * time.Minute)
	r.Tick()

	got := rem.reapedList()
	if len(got) != 1 || got[0] != "play-1" {
		t.Fatalf("only play-* boxes should be reaped, got %v", got)
	}
	if r.Info("smoke").Idle {
		t.Fatalf("out-of-scope box must not be tracked")
	}
}

func TestVanishedInstancePruned(t *testing.T) {
	src := &fakeSource{}
	rem := &fakeRemover{}
	c := &clock{t: time.Unix(1_000_000, 0)}
	r := newTestReaper(Config{IdleTimeout: 10 * time.Minute, NamePrefix: "play-"}, src, rem, c)

	src.set(Snapshot{Instance: "play-gone", Active: false})
	r.Tick()
	if !r.Info("play-gone").Idle {
		t.Fatalf("expected tracking after first pass")
	}

	// Instance disappears from the snapshot (removed elsewhere).
	src.set()
	r.Tick()
	if r.Info("play-gone").Idle {
		t.Fatalf("vanished instance should be pruned from tracking")
	}
	if got := rem.reapedList(); len(got) != 0 {
		t.Fatalf("a vanished instance is not our reap, got %v", got)
	}
}

func TestInfoWarningWindow(t *testing.T) {
	src := &fakeSource{}
	rem := &fakeRemover{}
	c := &clock{t: time.Unix(1_000_000, 0)}
	r := newTestReaper(Config{IdleTimeout: 10 * time.Minute, WarnBefore: 2 * time.Minute, NamePrefix: "play-"}, src, rem, c)

	src.set(Snapshot{Instance: "play-w", Active: false})
	r.Tick() // start clock at t0

	// t+7m: reapAt is t0+10m, warn window is last 2m (from t0+8m) → not yet.
	c.add(7 * time.Minute)
	if info := r.Info("play-w"); info.Warning {
		t.Fatalf("should not be in the warning window at t+7m")
	}
	// t+9m: inside the last 2 minutes → warning.
	c.add(2 * time.Minute)
	info := r.Info("play-w")
	if !info.Warning {
		t.Fatalf("should be in the warning window at t+9m")
	}
	if info.ReapAt.IsZero() || !info.Idle {
		t.Fatalf("warning Info should carry a reap time: %+v", info)
	}
}

func TestDisabledReaperNoOps(t *testing.T) {
	src := &fakeSource{}
	rem := &fakeRemover{}
	c := &clock{t: time.Unix(1_000_000, 0)}
	r := newTestReaper(Config{IdleTimeout: 0, NamePrefix: "play-"}, src, rem, c) // disabled

	if r.Enabled() {
		t.Fatalf("zero timeout should disable the reaper")
	}
	src.set(Snapshot{Instance: "play-a", Active: false})
	for i := 0; i < 5; i++ {
		c.add(1 * time.Hour)
		r.Tick()
	}
	if got := rem.reapedList(); len(got) != 0 {
		t.Fatalf("disabled reaper must never reap, got %v", got)
	}
	if r.Info("play-a").Idle {
		t.Fatalf("disabled reaper reports no idle info")
	}
}

func TestReapFailureKeepsGoing(t *testing.T) {
	src := &fakeSource{}
	rem := &fakeRemover{failFor: map[string]error{"play-bad": errBoom}}
	c := &clock{t: time.Unix(1_000_000, 0)}
	r := newTestReaper(Config{IdleTimeout: 1 * time.Minute, NamePrefix: "play-"}, src, rem, c)

	src.set(
		Snapshot{Instance: "play-bad", Active: false},
		Snapshot{Instance: "play-ok", Active: false},
	)
	r.Tick()
	c.add(5 * time.Minute)
	r.Tick()

	// play-ok still reaped even though play-bad errored.
	got := rem.reapedList()
	found := false
	for _, g := range got {
		if g == "play-ok" {
			found = true
		}
	}
	if !found {
		t.Fatalf("a failing reap must not block the others, got %v", got)
	}
}

var errBoom = &boomErr{}

type boomErr struct{}

func (*boomErr) Error() string { return "boom" }
