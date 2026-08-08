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
