package hostrunner

import "testing"

// REGRESSION (the FROZEN PANEL): the admin diagnostics panel read the host Registry's
// last RunnerEvent, which only exists while a host runner is ATTACHED and ticking. On a
// box with host-running disabled (or merely observed) no event was ever emitted, so the
// panel sat frozen at tick 0 — while the navfp log, which comes from the reader, updated
// fine. The endpoint now overlays the CURRENT tick's ScraperReadout, so the live reads
// flow with NO runner attached at all.
func TestDiagnosticsLiveWithNoRunnerAttached(t *testing.T) {
	reg := NewRegistry(nil)
	d := reg.Diagnostics("pod1") // nothing attached, no events → the frozen zero snapshot
	if d.Tick != 0 || d.Dela != "" {
		t.Fatalf("precondition: registry-only snapshot should be empty, got %+v", d)
	}

	// The scraper publishes a readout every tick regardless of the runner.
	ro := ScraperReadout{
		Fresh: true, Tick: 4242, GameState: string(PhaseMenu), MenuActive: true,
		GameConnection: 1, MenuItem: int(MenuItemSystemLink),
		Dela:      `ui\shell\main_menu\multiplayer_type_select\multiplayer_type_conn_item`,
		MenuFocus: 0x80046578, UIWidgetBlocks: 168, UIHighlighted: 3, UIMaxTick: 0x11BB71,
		MapCursor: 5, MapCursorCount: 36, MapCursorValid: true,
	}
	d.ApplyReadout(ro)

	if d.Tick != 4242 {
		t.Errorf("tick must come from the LIVE readout, got %d (frozen at 0?)", d.Tick)
	}
	if d.Dela != ro.Dela {
		t.Errorf("dela = %q, want the live readout's path", d.Dela)
	}
	if d.MenuFocus != 0x80046578 || d.UIWidgetBlocks != 168 || d.UIHighlighted != 3 {
		t.Errorf("cold-boot candidates not live: focus=0x%X blocks=%d hl=%d",
			d.MenuFocus, d.UIWidgetBlocks, d.UIHighlighted)
	}
	if !d.TreeBuilt {
		t.Error("tree_built should be derived true from a populated dela/focus/highlight")
	}
	if d.MapCursor.Index != 5 || d.MapCursor.Count != 36 || !d.MapCursor.Valid {
		t.Errorf("map cursor not live: %+v", d.MapCursor)
	}
	if d.GameConnection != 1 {
		t.Errorf("game_connection = %d, want the live 1", d.GameConnection)
	}
	// Screen is CLASSIFIED from the same readout, matching what the runner would see.
	if d.Screen != Classify(ro.Observation()).String() {
		t.Errorf("screen = %q, want the classification of the live readout", d.Screen)
	}
}

// Successive readouts must keep moving the panel (it's per-tick, not a one-time snapshot).
func TestDiagnosticsTracksSuccessiveTicks(t *testing.T) {
	var d Diagnostics
	d.ApplyReadout(ScraperReadout{Fresh: true, Tick: 1, Dela: "a", MapCursor: 1, MapCursorCount: 36, MapCursorValid: true})
	first := d
	d.ApplyReadout(ScraperReadout{Fresh: true, Tick: 2, Dela: "b", MapCursor: 2, MapCursorCount: 36, MapCursorValid: true})
	if d.Tick == first.Tick || d.Dela == first.Dela || d.MapCursor.Index == first.MapCursor.Index {
		t.Fatalf("panel did not advance across ticks: %+v then %+v", first, d)
	}
}
