package hostrunner

import (
	"testing"
	"time"
)

// on-screen observation helpers for the hosting card screens (distinct by which
// list widget is up).
func mapSelectObs(cur int) Observation {
	return Observation{Fresh: true, Phase: PhaseMenu, Connection: ConnHosting,
		MapCursor: cur, MapCursorCount: 36, MapCursorValid: true}
}
func gametypeSelectObs(cur int) Observation {
	return Observation{Fresh: true, Phase: PhaseMenu, Connection: ConnHosting,
		GametypeCursor: cur, GametypeCursorCount: 26, GametypeCursorValid: true}
}

// SAFETY: B on SELECT MAP would end the live lobby. guardDestructiveBack must block
// it there and pass it on the SAFE screens (pregame lobby, SELECT GAMETYPE).
func TestGuardRefusesBackOnMapSelect(t *testing.T) {
	b := tap("b", "back", "x")
	if got := guardDestructiveBack(b, mapSelectObs(3)); got.Kind != ActionBlocked {
		t.Fatalf("B on SELECT MAP must be blocked, got %v (%s)", got.Kind, got.Reason)
	}
	if got := guardDestructiveBack(b, gametypeSelectObs(0)); got.Kind != ActionTap {
		t.Fatalf("B on SELECT GAMETYPE is safe, must pass, got %v", got.Kind)
	}
	if got := guardDestructiveBack(b, lobby()); got.Kind != ActionTap {
		t.Fatalf("B on the pregame lobby is safe, must pass, got %v", got.Kind)
	}
	// A non-B key on map-select is fine (the drive keys A / Right / Left).
	if got := guardDestructiveBack(tap("Right", "nav", "x"), mapSelectObs(3)); got.Kind != ActionTap {
		t.Fatalf("non-B keys must pass on SELECT MAP, got %v", got.Kind)
	}
}

// SAFETY backstop: even if a path reaches execute() with a B on SELECT MAP, the key
// is NEVER sent to Input; non-B keys and B off-map-select ARE sent.
func TestExecuteInterlockNeverSendsBOnMapSelect(t *testing.T) {
	in := &fakeInput{}
	r := New(Config{Instance: "pod1"}, in, nil)

	r.lastObs = mapSelectObs(3) // on SELECT MAP
	_ = r.execute(tap("b", "back", "x"))
	for _, tp := range in.taps {
		if tp == "b" {
			t.Fatal("execute() sent B on SELECT MAP — would kill the live lobby")
		}
	}
	_ = r.execute(tap("a", "commit", "x")) // non-B sent
	if len(in.taps) != 1 || in.taps[0] != "a" {
		t.Fatalf("non-B key should be sent, taps=%v", in.taps)
	}
	r.lastObs = lobby() // B off map-select IS sent
	_ = r.execute(tap("b", "back", "x"))
	if in.taps[len(in.taps)-1] != "b" {
		t.Fatalf("B off SELECT MAP should be sent, taps=%v", in.taps)
	}
}

// SAFETY: the operator WalkBack must also refuse B on SELECT MAP.
func TestWalkBackRefusedOnMapSelect(t *testing.T) {
	in := &fakeInput{}
	r := New(Config{Instance: "pod1"}, in, nil)
	r.WalkBack(mapSelectObs(3), time.Unix(1000, 0))
	for _, tp := range in.taps {
		if tp == "b" {
			t.Fatal("WalkBack sent B on SELECT MAP — would kill the live lobby")
		}
	}
}

// RE-SELECT: a new pick after the lobby is created backs out (B on lobby → B on
// gametype-select) and re-drives FORWARD on SELECT MAP — never B on SELECT MAP.
func TestReselectBacksOutSafelyAndRedrives(t *testing.T) {
	in := &fakeInput{}
	sel := NewAtomicSelector()
	r := New(Config{Instance: "pod1", Selector: sel}, in, nil)
	t0 := time.Unix(1000, 0)

	// Initial pick → the sequence reaches the created lobby (catch-up via lobby()).
	sel.Set(Pick{Name: "Prisoner", Steps: 1}, Pick{Name: "Slayer", Steps: 0})
	if a := r.Tick(lobby(), t0); a.Kind != ActionWait {
		t.Fatalf("initial: should settle at lobby, got %v (%s)", a.Kind, a.Reason)
	}
	if a := r.Tick(lobby(), t0.Add(time.Second)); a.Kind != ActionWait {
		t.Fatalf("initial: should idle at lobby, got %v", a.Kind)
	}

	// NEW pick → re-select begins with a SAFE back-out B on the lobby.
	sel.Set(Pick{Name: "Temple", Steps: 7}, Pick{Name: "CTF", Steps: 2})
	if a := r.Tick(lobby(), t0.Add(2*time.Second)); a.Kind != ActionTap || a.Key() != "b" {
		t.Fatalf("new pick → back-out B on lobby, got %v key=%q (%s)", a.Kind, a.Key(), a.Reason)
	}

	// On SELECT GAMETYPE → second SAFE back-out B (map list still down).
	if a := r.Tick(gametypeSelectObs(0), t0.Add(3*time.Second)); a.Kind != ActionTap || a.Key() != "b" {
		t.Fatalf("gametype-select → back-out B, got %v key=%q", a.Kind, a.Key())
	}

	// On SELECT MAP → must DRIVE toward the new target (Right/Left), NEVER B.
	a := r.Tick(mapSelectObs(0), t0.Add(4*time.Second))
	if a.Kind != ActionTap || (a.Key() != "Right" && a.Key() != "Left") {
		t.Fatalf("map-select → drive toward new target, got %v key=%q (%s)", a.Kind, a.Key(), a.Reason)
	}
	if a.Key() == "b" {
		t.Fatal("re-drive must never press B on SELECT MAP")
	}
	// No B was ever sent while on SELECT MAP.
	for _, tp := range in.taps {
		if tp == "b" && a.Key() == "b" {
			t.Fatal("stray B on SELECT MAP")
		}
	}
}

// Drive the re-select END-TO-END (back out, re-drive map+gametype, reach lobby) and
// confirm it SETTLES to idle — it does not re-trigger with no further pick change.
func TestReselectSettlesAtLobby(t *testing.T) {
	in := &fakeInput{}
	sel := NewAtomicSelector()
	r := New(Config{Instance: "pod1", Selector: sel}, in, nil)
	tk := time.Unix(1000, 0)
	step := func(o Observation) Action { tk = tk.Add(1500 * time.Millisecond); return r.Tick(o, tk) }

	sel.Set(Pick{Name: "Prisoner", Steps: 1}, Pick{Name: "Slayer", Steps: 0})
	step(lobby()) // initial → created lobby (appliedGen=1)

	sel.Set(Pick{Name: "Temple", Steps: 7}, Pick{Name: "CTF", Steps: 2})
	step(lobby())              // begin re-select: B back-out to gametype
	step(gametypeSelectObs(0)) // B back-out to map
	step(mapSelectObs(7))      // cursor==target(7) → A commit map
	step(mapSelectObs(7))      // after BlindAdvanceAfter → advance to gametype re-drive
	step(gametypeSelectObs(2)) // cursor==target(2) → A commit gametype
	step(gametypeSelectObs(2)) // after BlindAdvanceAfter → advance to reach-lobby

	// Settled lobby, no new pick → idle (not another back-out).
	if a := step(lobby()); a.Kind != ActionWait {
		t.Fatalf("re-select should settle to idle at the lobby, got %v key=%q (%s)", a.Kind, a.Key(), a.Reason)
	}
}

// REGRESSION (live, 2026-08-11): a MAP-ONLY re-pick hung on SELECT GAMETYPE. The
// sequence advances onto the gametype card on BlindAdvanceAfter — inside the
// map→gametype screen-transition window where the gametype list isn't up yet
// (count 0) — and during a RE-SELECT the PREVIOUS commit's map+gametype are still
// resident, so the old "finalized pick resident" test advanced PAST the card
// without pressing its A; the reach-lobby sentinel then waited forever. The
// count-0 advance now requires the settled-lobby SCREEN RECORD; the transition
// tick (card-wrapper record + stale residents) must HOLD, and the gametype card —
// whose cursor already rests ON the unchanged pick — must still commit its A.
func TestReselectMapOnlyStillCommitsGametypeA(t *testing.T) {
	const (
		mapScreen      = `ui\shell\main_menu\multiplayer_type_select\connected\connected_map_select_wrapper`
		gametypeScreen = `ui\shell\main_menu\multiplayer_type_select\connected\connected_gametype_select_wrapper`
		pregameScreen  = `ui\shell\main_menu\multiplayer_type_select\connected\pregame\connected_pregame_screen`
	)
	withResidents := func(o Observation, screen string) Observation {
		o.Map, o.Gametype = "bloodgulch", "slayer" // the PREVIOUS commit, still resident
		o.UiScreen = screen
		return o
	}
	sel := NewAtomicSelector()
	sel.Set(Pick{Name: "wizard", Steps: 2}, Pick{Name: "slayer", Steps: 0}) // map changed, gametype unchanged
	s := ReselectSequence(DefaultTiming, sel)
	t0 := time.Unix(1000, 0)

	// Safe back-out: B on the lobby, then B on SELECT GAMETYPE.
	lob := withResidents(lobby(), pregameScreen)
	if a := s.Step(lob, t0); a.Key() != "b" {
		t.Fatalf("back-out 1 should press B on the lobby, got %v", a)
	}
	if a := s.Step(withResidents(gametypeSelectObs(0), gametypeScreen), t0.Add(time.Second)); a.Key() != "b" {
		t.Fatalf("back-out 2 should press B on SELECT GAMETYPE, got %v", a)
	}
	// Re-drive the map: cursor 0 → target 2, then commit.
	if a := s.Step(withResidents(mapSelectObs(0), mapScreen), t0.Add(2*time.Second)); a.Key() != "Right" {
		t.Fatalf("map re-drive should press Right, got %v", a)
	}
	if a := s.Step(withResidents(mapSelectObs(2), mapScreen), t0.Add(3*time.Second)); a.Key() != "a" {
		t.Fatalf("map on target should commit A, got %v", a)
	}
	// THE RACE TICK: the sequence advances onto the gametype card on the blind
	// timer while the screen is still transitioning — no list up (count 0), the
	// OLD map+gametype resident, the record on a card wrapper. Must HOLD.
	transition := withResidents(Observation{Fresh: true, Phase: PhasePreGame, Connection: ConnHosting}, gametypeScreen)
	raceAt := t0.Add(3*time.Second + DefaultTiming.BlindAdvanceAfter)
	if a := s.Step(transition, raceAt); a.Kind != ActionWait {
		t.Fatalf("count-0 transition with stale residents must HOLD, got %v (%s)", a.Kind, a.Reason)
	}
	if cur, _ := s.Current(); cur.Name != "reselect-gametype" {
		t.Fatalf("sequence skipped the gametype card during the transition (now %q) — the live hang", cur.Name)
	}
	// The gametype list comes up with the cursor already ON the unchanged pick —
	// the card must STILL press A to leave the screen.
	if a := s.Step(withResidents(gametypeSelectObs(0), gametypeScreen), raceAt.Add(300*time.Millisecond)); a.Kind != ActionTap || a.Key() != "a" {
		t.Fatalf("unchanged gametype on target must still commit A, got %v (%s)", a.Kind, a.Reason)
	}
	// A committed → settled lobby (pregame record) → done.
	done := s.Step(withResidents(lobby(), pregameScreen), raceAt.Add(300*time.Millisecond+DefaultTiming.BlindAdvanceAfter))
	if done.Kind != ActionDone {
		t.Fatalf("after the gametype A the re-select should settle at the lobby, got %v (%s)", done.Kind, done.Reason)
	}
}

// The record-less fallback must still let the catch-up complete for a settled box
// (no ui_screen, finalized map+gametype resident) — the pre-record behavior.
func TestCardCatchUpFallbackWithoutRecord(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	if a := s.Step(lobby(), time.Unix(1000, 0)); a.Kind != ActionDone {
		t.Fatalf("settled record-less box should catch up to done, got %v (%s)", a.Kind, a.Reason)
	}
}
