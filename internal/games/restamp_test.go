package games_test

import (
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/tests"

	"github.com/Stewball32/xemu-cartographer/internal/games"
)

// TestRestampEvents (§7.2): a late in-window row inserted after the persist
// is stamped by RestampEvents; rows of the next game on the same instance
// (after the window) and of other instances stay untouched; a repeat call
// stamps nothing new.
func TestRestampEvents(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureCollections(t, app)

	early := insertEvent(t, app, "pod-a", winMid, "")
	res, err := games.PersistFinishedGame(app, sampleGame())
	if err != nil {
		t.Fatalf("PersistFinishedGame: %v", err)
	}
	if res.EventsStamped != 1 || reload(t, app, early.Id).GetString("game") != res.GameID {
		t.Fatalf("persist stamped %d (early game=%q), want 1", res.EventsStamped, reload(t, app, early.Id).GetString("game"))
	}

	// The sink delivers a late in-window row, a next-game row (after the
	// window) and a row on another instance after the persist ran.
	late := insertEvent(t, app, "pod-a", winEnd.Add(-time.Second), "")
	next := insertEvent(t, app, "pod-a", after, "")
	other := insertEvent(t, app, "pod-b", winMid, "")

	n, err := games.RestampEvents(app, res.GameID, "pod-a", sampleGame().StartedAt, sampleGame().EndedAt)
	if err != nil {
		t.Fatalf("RestampEvents: %v", err)
	}
	if n != 1 {
		t.Fatalf("RestampEvents stamped %d, want 1", n)
	}
	if g := reload(t, app, late.Id).GetString("game"); g != res.GameID {
		t.Errorf("late in-window row game = %q, want %q", g, res.GameID)
	}
	if g := reload(t, app, early.Id).GetString("game"); g != res.GameID {
		t.Errorf("already stamped row changed to %q", g)
	}
	for name, id := range map[string]string{"next-game": next.Id, "other-instance": other.Id} {
		if g := reload(t, app, id).GetString("game"); g != "" {
			t.Errorf("%s row should be unstamped, got game=%q", name, g)
		}
	}

	// Idempotent.
	if n, err := games.RestampEvents(app, res.GameID, "pod-a", sampleGame().StartedAt, sampleGame().EndedAt); err != nil || n != 0 {
		t.Errorf("second RestampEvents = %d, %v; want 0, nil", n, err)
	}
	// Guards: no game / no instance / no app stamp nothing.
	if n, err := games.RestampEvents(app, "", "pod-a", time.Time{}, time.Time{}); err != nil || n != 0 {
		t.Errorf("empty gameID = %d, %v", n, err)
	}
	if n, err := games.RestampEvents(nil, res.GameID, "pod-a", time.Time{}, time.Time{}); err != nil || n != 0 {
		t.Errorf("nil app = %d, %v", n, err)
	}
	if g := reload(t, app, next.Id).GetString("game"); g != "" {
		t.Errorf("next-game row stamped by a guarded call: %q", g)
	}
}
