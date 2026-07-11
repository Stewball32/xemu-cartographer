package hostrunner

import (
	"testing"
	"time"
)

// A player hitting "ready" (SetReady true) flips the default arm-only runner into
// arm+start: it presses A to start once native conditions pass.
func TestRunnerPlayerReadyStarts(t *testing.T) {
	in := &fakeInput{}
	// Default config = arm-only. Give it a mutable selector so SetSelection works.
	r := New(Config{Instance: "pod1", Selector: NewAtomicSelector(Pick{}, Pick{})}, in, nil)
	t0 := time.Unix(1000, 0)

	// Walk to a ready lobby (2 boxes, 2 teams) while NOT ready → holds (arm-only).
	last := r.Tick(lobby(), t0)
	if last.Kind != ActionWait {
		t.Fatalf("arm-only should hold at lobby, got %v (%s)", last.Kind, last.Reason)
	}
	for _, k := range in.taps {
		if k == "a" && last.Intent == "start countdown" {
			t.Fatal("must not start before player readies")
		}
	}

	// Player readies → next tick presses start.
	r.SetReady(true)
	if !r.Ready() {
		t.Fatal("Ready() should report true after SetReady(true)")
	}
	last = r.Tick(lobby(), t0.Add(time.Second))
	if last.Kind != ActionTap || last.Key() != "a" || last.Intent != "start countdown" {
		t.Fatalf("ready player should start (tap A), got %v (%s)", last.Kind, last.Reason)
	}
}

// Unready (SetReady false) keeps a default runner arm-only even at a ready lobby.
func TestRunnerPlayerUnreadyHolds(t *testing.T) {
	in := &fakeInput{}
	r := New(Config{Instance: "pod1", Selector: NewAtomicSelector(Pick{}, Pick{})}, in, nil)
	r.SetReady(true)
	r.SetReady(false)
	last := r.Tick(lobby(), time.Unix(1000, 0))
	if last.Kind != ActionWait {
		t.Fatalf("unready player should hold arm-only, got %v (%s)", last.Kind, last.Reason)
	}
}

// SetSelection records the pick on a mutable selector and Selection reads it back.
func TestRunnerSelection(t *testing.T) {
	r := New(Config{Instance: "pod1", Selector: NewAtomicSelector(Pick{}, Pick{})}, nil, nil)
	if !r.SetSelection("Blood Gulch", "Team Slayer") {
		t.Fatal("SetSelection should succeed on an AtomicSelector")
	}
	mp, gt := r.Selection()
	if mp.Name != "Blood Gulch" || gt.Name != "Team Slayer" {
		t.Fatalf("Selection = %q/%q, want Blood Gulch/Team Slayer", mp.Name, gt.Name)
	}
}

// A non-mutable (default) selector rejects SetSelection.
func TestRunnerSelectionImmutable(t *testing.T) {
	r := New(Config{Instance: "pod1"}, nil, nil) // DefaultSelector (FixedSelector)
	if r.SetSelection("Blood Gulch", "Slayer") {
		t.Fatal("SetSelection should return false for a non-mutable selector")
	}
}

// The Registry exposes the play-control surface and the enriched Status.
func TestRegistryPlayControls(t *testing.T) {
	reg := NewRegistry(nil)
	r := New(Config{Instance: "pod1", Selector: NewAtomicSelector(Pick{}, Pick{})}, nil, reg)
	reg.Register("pod1", r)

	if !reg.SetSelection("pod1", "Sidewinder", "Slayer") {
		t.Fatal("SetSelection via registry should succeed")
	}
	if !reg.SetReady("pod1", true) {
		t.Fatal("SetReady via registry should succeed")
	}

	// Drive one tick so the last-event fields populate.
	r.Tick(lobby(), time.Unix(1000, 0))
	st := reg.Status("pod1")
	if !st.Present || !st.Ready {
		t.Fatalf("status should show present+ready: %+v", st)
	}
	if st.SelectedMap != "Sidewinder" || st.SelectedGametype != "Slayer" {
		t.Fatalf("status selection = %q/%q, want Sidewinder/Slayer", st.SelectedMap, st.SelectedGametype)
	}

	// Unknown instance → false.
	if reg.SetReady("nope", true) || reg.SetSelection("nope", "x", "y") {
		t.Fatal("play controls on an unknown instance must return false")
	}
}

func TestDefaultCatalog(t *testing.T) {
	c := DefaultCatalog()
	if len(c.Maps) == 0 || len(c.Gametypes) == 0 {
		t.Fatal("catalog should be non-empty")
	}
}
