package hostrunner

import (
	"testing"
	"time"
)

// obs helpers.
func systemLink() Observation {
	return Observation{Fresh: true, Phase: PhaseMenu, Connection: ConnSystemLink}
}
func hosting() Observation {
	return Observation{Fresh: true, Phase: PhaseMenu, Connection: ConnHosting}
}
func lobby() Observation {
	return Observation{Fresh: true, Phase: PhaseMenu, Connection: ConnHosting, MachineCount: 2, TeamCount: 2, Map: "bloodgulch", Gametype: "slayer"}
}
func mainMenu() Observation {
	return Observation{Fresh: true, Phase: PhaseMenu, MenuActive: true, Connection: ConnMenu}
}

// proceedSelector is a non-parking selection (default cards, 0 D-pad steps) so
// the sequence flow tests exercise the press mechanics without the park gate.
func proceedSelector() Selector {
	return FixedSelector{Map: Pick{Name: "default"}, Gametype: Pick{Name: "default"}}
}

// TestSequenceParksAtMapSelectUntilPick: with no selection the runner presses Y
// (creates + advertises the lobby) then HOLDS at map-select — it must NOT press A
// on a blind default. Once a pick arrives it applies it in the forward pass.
func TestSequenceParksAtMapSelectUntilPick(t *testing.T) {
	sel := NewAtomicSelector()
	s := DefaultHostSequence(DefaultTiming, sel)
	t0 := time.Unix(1000, 0)

	if a := s.Step(systemLink(), t0); a.Kind != ActionTap || a.Key() != "y" {
		t.Fatalf("first press should be Y (create+advertise lobby), got %v", a)
	}
	// Reached map-select (hosting) with NO pick → park, never press A.
	for i := 0; i < 5; i++ {
		a := s.Step(hosting(), t0.Add(time.Duration(1+i)*time.Second))
		if a.Kind == ActionTap {
			t.Fatalf("must not press while parked (tick %d), got tap %q", i, a.Key())
		}
		if a.Kind != ActionWait {
			t.Fatalf("parked step should wait, got %v (%s)", a.Kind, a.Reason)
		}
	}
	// Player picks → forward pass presses A (default card, 0 steps).
	sel.Set(Pick{Name: "bloodgulch"}, Pick{Name: "slayer"})
	a := s.Step(hosting(), t0.Add(10*time.Second))
	if a.Kind != ActionTap || a.Key() != "a" || a.Intent != "select map" {
		t.Fatalf("after pick should press A (select map), got %v (%s)", a.Kind, a.Reason)
	}
}

// TestSequenceNavigatesToChosenCard: a pick with D-pad Steps walks the carousel
// (Right × steps) before pressing A, for both the map and gametype cards.
func TestSequenceNavigatesToChosenCard(t *testing.T) {
	sel := NewAtomicSelector()
	sel.Set(Pick{Name: "wizard", Steps: 2}, Pick{Name: "koth", Steps: 1})
	s := DefaultHostSequence(DefaultTiming, sel)
	t0 := time.Unix(1000, 0)

	s.Step(systemLink(), t0) // Y
	// Map card: 2 Right presses, then A.
	if a := s.Step(hosting(), t0.Add(1*time.Second)); a.Key() != "Right" {
		t.Fatalf("map nav 1 should be Right, got %v", a)
	}
	if a := s.Step(hosting(), t0.Add(2*time.Second)); a.Key() != "Right" {
		t.Fatalf("map nav 2 should be Right, got %v", a)
	}
	if a := s.Step(hosting(), t0.Add(3*time.Second)); a.Key() != "a" || a.Intent != "select map" {
		t.Fatalf("after 2 nav should press A (map), got %v", a)
	}
	// Gametype card after the blind timer: 1 Right, then A.
	if a := s.Step(hosting(), t0.Add(3*time.Second+DefaultTiming.BlindAdvanceAfter)); a.Key() != "Right" {
		t.Fatalf("gametype nav should be Right, got %v", a)
	}
	if a := s.Step(hosting(), t0.Add(6*time.Second)); a.Key() != "a" || a.Intent != "select gametype" {
		t.Fatalf("after gametype nav should press A, got %v", a)
	}
}

// TestSequenceFullFlow walks the host flow end-to-end with a controlled clock,
// asserting each gated press fires on the right screen and confirms before the
// next.
func TestSequenceFullFlow(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	t0 := time.Unix(1000, 0)

	// On system link, first tick presses Y (create game).
	a := s.Step(systemLink(), t0)
	if a.Kind != ActionTap || a.Key() != "y" {
		t.Fatalf("step1: got %v key=%q, want tap y", a.Kind, a.Key())
	}
	// Still on system link, before RepressAfter → wait (awaiting confirm), no re-press yet.
	a = s.Step(systemLink(), t0.Add(300*time.Millisecond))
	if a.Kind != ActionWait {
		t.Fatalf("step2: got %v, want wait", a.Kind)
	}

	// Entered hosting (Y landed): create-game.Done(hosting) true → advance;
	// select-map is blind → press A.
	a = s.Step(hosting(), t0.Add(1*time.Second))
	if a.Kind != ActionTap || a.Key() != "a" || a.Intent != "select map" {
		t.Fatalf("step3: got %v key=%q intent=%q, want tap a (select map)", a.Kind, a.Key(), a.Intent)
	}

	// Still hosting, before BlindAdvanceAfter → wait.
	a = s.Step(hosting(), t0.Add(1200*time.Millisecond))
	if a.Kind != ActionWait {
		t.Fatalf("step4: got %v, want wait (blind pending)", a.Kind)
	}

	// After BlindAdvanceAfter → advance to select-gametype, press A again.
	a = s.Step(hosting(), t0.Add(1*time.Second+DefaultTiming.BlindAdvanceAfter))
	if a.Kind != ActionTap || a.Key() != "a" || a.Intent != "select gametype" {
		t.Fatalf("step5: got %v key=%q intent=%q, want tap a (select gametype)", a.Kind, a.Key(), a.Intent)
	}

	// Landed in the lobby: all remaining steps' Done(inLobby) hold → Done.
	a = s.Step(lobby(), t0.Add(3*time.Second))
	if a.Kind != ActionDone {
		t.Fatalf("step6: got %v (%s), want done", a.Kind, a.Reason)
	}
	if !s.Done() {
		t.Error("sequence should be complete")
	}
}

// TestSequenceBlocksOnWrongScreen: a non-blind step never presses blindly.
func TestSequenceBlocksOnWrongScreen(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	// On the main menu, create-game (expects system link) must NOT press Y.
	a := s.Step(mainMenu(), time.Unix(1000, 0))
	if a.Kind != ActionBlocked {
		t.Fatalf("got %v (%s), want blocked on wrong screen", a.Kind, a.Reason)
	}
}

// TestSequenceRepress: a non-blind step re-presses after RepressAfter if the
// press didn't land.
func TestSequenceRepress(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	t0 := time.Unix(1000, 0)
	if a := s.Step(systemLink(), t0); a.Key() != "y" {
		t.Fatal("expected first Y press")
	}
	if a := s.Step(systemLink(), t0.Add(500*time.Millisecond)); a.Kind != ActionWait {
		t.Fatal("expected wait before repress window")
	}
	a := s.Step(systemLink(), t0.Add(DefaultTiming.RepressAfter+time.Millisecond))
	if a.Kind != ActionTap || a.Key() != "y" {
		t.Fatalf("expected Y re-press after RepressAfter, got %v", a)
	}
}

// TestSequenceCatchUp: if we're already in the lobby (presses landed fast), the
// sequence reports Done without pressing anything.
func TestSequenceCatchUp(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	a := s.Step(lobby(), time.Unix(1000, 0))
	if a.Kind != ActionDone {
		t.Fatalf("got %v, want done (already in lobby)", a.Kind)
	}
}

func TestWalkBackCancelsCountdown(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	// advance the cursor a couple steps so we can observe it does NOT rewind
	// while a countdown is active.
	s.cursor = 3
	obs := lobby()
	obs.CountdownActive = true
	a := s.WalkBack(obs, time.Unix(1000, 0))
	if a.Kind != ActionTap || a.Key() != "b" || a.Intent != "cancel countdown" {
		t.Fatalf("got %v key=%q intent=%q, want tap b cancel countdown", a.Kind, a.Key(), a.Intent)
	}
	if s.cursor != 3 {
		t.Errorf("cancel countdown must not rewind cursor, got %d", s.cursor)
	}
}

func TestWalkBackRewinds(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	s.cursor = 2
	a := s.WalkBack(lobby(), time.Unix(1000, 0))
	if a.Kind != ActionTap || a.Key() != "b" || a.Intent != "walk back" {
		t.Fatalf("got %v key=%q, want tap b walk back", a.Kind, a.Key())
	}
	if s.cursor != 1 {
		t.Errorf("walk back should rewind cursor to 1, got %d", s.cursor)
	}
}

func TestSequenceStaleNoAction(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	if a := s.Step(Observation{Fresh: false}, time.Unix(1000, 0)); a.Kind != ActionWait {
		t.Fatalf("stale obs should wait, got %v", a.Kind)
	}
}
