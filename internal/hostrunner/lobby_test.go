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

// TestSequenceFullFlow walks the host flow end-to-end with a controlled clock,
// asserting each gated press fires on the right screen and confirms before the
// next.
func TestSequenceFullFlow(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming)
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
	s := DefaultHostSequence(DefaultTiming)
	// On the main menu, create-game (expects system link) must NOT press Y.
	a := s.Step(mainMenu(), time.Unix(1000, 0))
	if a.Kind != ActionBlocked {
		t.Fatalf("got %v (%s), want blocked on wrong screen", a.Kind, a.Reason)
	}
}

// TestSequenceRepress: a non-blind step re-presses after RepressAfter if the
// press didn't land.
func TestSequenceRepress(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming)
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
	s := DefaultHostSequence(DefaultTiming)
	a := s.Step(lobby(), time.Unix(1000, 0))
	if a.Kind != ActionDone {
		t.Fatalf("got %v, want done (already in lobby)", a.Kind)
	}
}

func TestWalkBackCancelsCountdown(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming)
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
	s := DefaultHostSequence(DefaultTiming)
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
	s := DefaultHostSequence(DefaultTiming)
	if a := s.Step(Observation{Fresh: false}, time.Unix(1000, 0)); a.Kind != ActionWait {
		t.Fatalf("stale obs should wait, got %v", a.Kind)
	}
}
