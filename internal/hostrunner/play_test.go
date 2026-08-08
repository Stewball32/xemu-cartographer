package hostrunner

import (
	"testing"
	"time"
)

// The runner's job ENDS at a created lobby with map + gametype selected — it never
// presses start (players start the match themselves). There is no ready gate and
// no 2-player/2-team start gate (Stewart 2026-08): even at a "ready" lobby, and
// even after a (now-vestigial) SetReady, the decision is a wait, never a start tap.
func TestRunnerStopsAtLobbyNeverStarts(t *testing.T) {
	in := &fakeInput{}
	r := New(Config{Instance: "pod1", Selector: NewAtomicSelector()}, in, nil)
	t0 := time.Unix(1000, 0)

	last := r.Tick(lobby(), t0) // reach a ready lobby (2 boxes, 2 teams)
	if last.Kind != ActionWait || last.Intent == "start countdown" {
		t.Fatalf("runner should wait at the lobby (never start), got %v (%s)", last.Kind, last.Reason)
	}

	// SetReady must NOT cause a start — the ready gate is gone.
	r.SetReady(true)
	last = r.Tick(lobby(), t0.Add(time.Second))
	if last.Kind != ActionWait || last.Intent == "start countdown" {
		t.Fatalf("runner must never press start (players start), got %v (%s)", last.Kind, last.Reason)
	}
}

// Unready (SetReady false) keeps a default runner arm-only even at a ready lobby.
func TestRunnerPlayerUnreadyHolds(t *testing.T) {
	in := &fakeInput{}
	r := New(Config{Instance: "pod1", Selector: NewAtomicSelector()}, in, nil)
	r.SetReady(true)
	r.SetReady(false)
	last := r.Tick(lobby(), time.Unix(1000, 0))
	if last.Kind != ActionWait {
		t.Fatalf("unready player should hold arm-only, got %v (%s)", last.Kind, last.Reason)
	}
}

// SetSelection records the picks (with nav Steps) and un-parks the runner;
// Selection/HasSelection read them back.
func TestRunnerSelection(t *testing.T) {
	r := New(Config{Instance: "pod1", Selector: NewAtomicSelector()}, nil, nil)
	if r.HasSelection() {
		t.Fatal("a fresh AtomicSelector runner should start unselected (parked)")
	}
	if !r.SetSelection(Pick{Name: "Blood Gulch", Steps: 3}, Pick{Name: "Team Slayer", Steps: 1}) {
		t.Fatal("SetSelection should succeed on an AtomicSelector")
	}
	if !r.HasSelection() {
		t.Fatal("HasSelection should be true after SetSelection")
	}
	mp, gt := r.Selection()
	if mp.Name != "Blood Gulch" || mp.Steps != 3 || gt.Name != "Team Slayer" || gt.Steps != 1 {
		t.Fatalf("Selection = %+v/%+v, want Blood Gulch@3 / Team Slayer@1", mp, gt)
	}
	// Clearing re-parks.
	if !r.ClearSelection() || r.HasSelection() {
		t.Fatal("ClearSelection should reset to unselected")
	}
}

// A non-mutable (default) selector rejects SetSelection.
func TestRunnerSelectionImmutable(t *testing.T) {
	r := New(Config{Instance: "pod1"}, nil, nil) // DefaultSelector (FixedSelector)
	if r.SetSelection(Pick{Name: "Blood Gulch"}, Pick{Name: "Slayer"}) {
		t.Fatal("SetSelection should return false for a non-mutable selector")
	}
}

// The Registry exposes the play-control surface and the enriched Status.
func TestRegistryPlayControls(t *testing.T) {
	reg := NewRegistry(nil)
	r := New(Config{Instance: "pod1", Selector: NewAtomicSelector()}, nil, reg)
	reg.Register("pod1", r)

	if !reg.SetSelection("pod1", "Sidewinder", 4, "Slayer", 0) {
		t.Fatal("SetSelection via registry should succeed")
	}
	if !reg.SetReady("pod1", true) {
		t.Fatal("SetReady via registry should succeed")
	}

	// Drive one tick so the last-event fields populate.
	r.Tick(lobby(), time.Unix(1000, 0))
	st := reg.Status("pod1")
	if !st.Present || !st.Ready || !st.Selected {
		t.Fatalf("status should show present+ready+selected: %+v", st)
	}
	if st.SelectedMap != "Sidewinder" || st.SelectedGametype != "Slayer" {
		t.Fatalf("status selection = %q/%q, want Sidewinder/Slayer", st.SelectedMap, st.SelectedGametype)
	}

	// ClearSelection re-parks.
	if !reg.ClearSelection("pod1") {
		t.Fatal("ClearSelection via registry should succeed")
	}
	if reg.Status("pod1").Selected {
		t.Fatal("status should show unselected after ClearSelection")
	}

	// Unknown instance → false.
	if reg.SetReady("nope", true) || reg.SetSelection("nope", "x", 0, "y", 0) || reg.ClearSelection("nope") {
		t.Fatal("play controls on an unknown instance must return false")
	}
}
