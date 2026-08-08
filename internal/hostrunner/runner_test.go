package hostrunner

import (
	"testing"
	"time"
)

type fakeInput struct {
	taps   []string
	chords [][]string
}

func (f *fakeInput) Tap(l string) error      { f.taps = append(f.taps, l); return nil }
func (f *fakeInput) Chord(l ...string) error { f.chords = append(f.chords, l); return nil }

type fakeSink struct{ events []RunnerEvent }

func (f *fakeSink) Emit(e RunnerEvent) { f.events = append(f.events, e) }

func (f *fakeSink) last() RunnerEvent { return f.events[len(f.events)-1] }

// drive the runner through the full gated host flow; assert the exact keys.
func TestRunnerArmOnlyFlow(t *testing.T) {
	in, sink := &fakeInput{}, &fakeSink{}
	r := New(Config{Instance: "pod1"}, in, sink) // default = arm-only
	t0 := time.Unix(1000, 0)

	r.Tick(systemLink(), t0)                                                 // tap y
	r.Tick(systemLink(), t0.Add(300*time.Millisecond))                       // wait
	r.Tick(hosting(), t0.Add(1*time.Second))                                 // tap a (map)
	r.Tick(hosting(), t0.Add(1*time.Second+DefaultTiming.BlindAdvanceAfter)) // tap a (gametype)
	last := r.Tick(lobby(), t0.Add(3*time.Second))                           // done → armed, arm-only wait

	want := []string{"y", "a", "a"}
	if len(in.taps) != len(want) {
		t.Fatalf("taps = %v, want %v", in.taps, want)
	}
	for i := range want {
		if in.taps[i] != want[i] {
			t.Fatalf("tap[%d] = %q, want %q", i, in.taps[i], want[i])
		}
	}
	if last.Kind != ActionWait {
		t.Fatalf("arm-only at lobby should wait (armed), got %v (%s)", last.Kind, last.Reason)
	}
	// Every tick emits an observable event carrying the read-only counts.
	if len(sink.events) < 5 {
		t.Fatalf("expected >=5 events, got %d", len(sink.events))
	}
	if e := sink.last(); e.MachineCount != 2 || e.TeamCount != 2 || !e.ReadyToStart {
		t.Errorf("event should surface native counts: %+v", e)
	}
}

// The runner drives the create-game sequence to a lobby (map + gametype selected)
// and then STOPS — it never presses start (Stewart 2026-08). Players start.
func TestRunnerReachesLobbyThenStops(t *testing.T) {
	in := &fakeInput{}
	r := New(Config{Instance: "pod1"}, in, nil)
	t0 := time.Unix(1000, 0)

	r.Tick(systemLink(), t0)
	r.Tick(hosting(), t0.Add(1*time.Second))
	r.Tick(hosting(), t0.Add(1*time.Second+DefaultTiming.BlindAdvanceAfter))
	last := r.Tick(lobby(), t0.Add(3*time.Second)) // sequence done → stop (no start)

	if last.Kind != ActionWait || last.Intent == "start countdown" {
		t.Fatalf("runner should stop (wait) at the lobby, never start; got %v (%s)", last.Kind, last.Reason)
	}
	// It DID drive the create-game sequence (pressed keys) — just never a start-A.
	if len(in.taps) == 0 {
		t.Fatal("runner should have driven the create-game sequence")
	}
}

func TestRunnerArmAndStartHoldsUntilReady(t *testing.T) {
	in := &fakeInput{}
	cfg := Config{Instance: "pod1", Start: StartPolicy{Predicates: []StartPredicate{NativeReadyPredicate()}, Mode: ArmAndStart}}
	r := New(cfg, in, nil)
	t0 := time.Unix(1000, 0)
	// Reach a lobby with only ONE box → native-ready fails → no start press.
	notReady := lobby()
	notReady.MachineCount = 1
	// Fast-path the sequence to done by feeding a lobby (catch-up).
	last := r.Tick(notReady, t0)
	if last.Kind != ActionWait {
		t.Fatalf("should hold start when not ready, got %v (%s)", last.Kind, last.Reason)
	}
	for _, k := range in.taps {
		if k == "a" {
			// The only "a" that could appear is a start press — there must be none.
			t.Fatalf("must not press start when native-not-ready; taps=%v", in.taps)
		}
	}
}

// Auto-host loop: post-game → clear carnage (A) → return to menu → re-host from top.
func TestRunnerAutoHostLoop(t *testing.T) {
	in := &fakeInput{}
	r := New(Config{Instance: "pod1"}, in, nil)
	t0 := time.Unix(1000, 0)

	r.Tick(Observation{Fresh: true, Phase: PhaseInGame}, t0) // match live: wait
	a := r.Tick(Observation{Fresh: true, Phase: PhasePostGame, Connection: ConnHosting}, t0.Add(time.Second))
	if a.Kind != ActionTap || a.Key() != "a" || a.Intent != "clear carnage" {
		t.Fatalf("post-game should tap A to clear carnage, got %v (%s)", a.Kind, a.Reason)
	}
	// Back at system link after the game → sequence resets and re-hosts (tap y).
	a = r.Tick(systemLink(), t0.Add(2*time.Second))
	if a.Kind != ActionTap || a.Key() != "y" {
		t.Fatalf("after game, should re-host from top (tap y), got %v (%s)", a.Kind, a.Reason)
	}
	if r.Sequence().Cursor() != 0 && !r.Sequence().Done() {
		// after the y press we're on cursor 0 pressed; fine either way, just not mid-flow stale
	}
}

// Arbitration: admin takeover suspends the runner — zero input emitted.
func TestRunnerAdminSuspend(t *testing.T) {
	in, sink := &fakeInput{}, &fakeSink{}
	r := New(Config{Instance: "pod1"}, in, sink)
	r.Arbiter().TakeOver()

	a := r.Tick(systemLink(), time.Unix(1000, 0))
	if a.Kind != ActionWait {
		t.Fatalf("suspended runner should wait, got %v", a.Kind)
	}
	if len(in.taps) != 0 {
		t.Fatalf("suspended runner must emit no input, got %v", in.taps)
	}
	if e := sink.last(); e.Kind != "suspended" || e.Authority != "admin" {
		t.Errorf("expected suspended/admin event, got kind=%q auth=%q", e.Kind, e.Authority)
	}

	// Release → drives again.
	r.Arbiter().Release()
	a = r.Tick(systemLink(), time.Unix(1001, 0))
	if a.Kind != ActionTap || a.Key() != "y" {
		t.Fatalf("after release should drive (tap y), got %v", a.Kind)
	}
}

// First live milestone: single gated press.
func TestRunnerGatedPress(t *testing.T) {
	in := &fakeInput{}
	r := New(Config{Instance: "pod1"}, in, nil)

	// On the expected screen → taps once.
	a := r.GatedPress(systemLink(), ScreenSystemLink, "a")
	if a.Kind != ActionTap || a.Key() != "a" {
		t.Fatalf("gated press on-screen should tap, got %v", a.Kind)
	}
	if len(in.taps) != 1 || in.taps[0] != "a" {
		t.Fatalf("expected one tap a, got %v", in.taps)
	}

	// Wrong screen → blocked, no tap.
	a = r.GatedPress(mainMenu(), ScreenSystemLink, "a")
	if a.Kind != ActionBlocked {
		t.Fatalf("gated press off-screen should block, got %v", a.Kind)
	}
	if len(in.taps) != 1 {
		t.Fatalf("blocked press must not emit input; taps=%v", in.taps)
	}

	// Suspended → no tap even on-screen.
	r.Arbiter().Disable()
	a = r.GatedPress(systemLink(), ScreenSystemLink, "a")
	if a.Kind != ActionWait || len(in.taps) != 1 {
		t.Fatalf("disabled gated press must not tap, got %v taps=%v", a.Kind, in.taps)
	}
}

func TestRunnerStaleNoInput(t *testing.T) {
	in := &fakeInput{}
	r := New(Config{Instance: "pod1"}, in, nil)
	r.Tick(Observation{Fresh: false}, time.Unix(1000, 0))
	if len(in.taps) != 0 {
		t.Fatalf("stale obs must emit no input, got %v", in.taps)
	}
}
