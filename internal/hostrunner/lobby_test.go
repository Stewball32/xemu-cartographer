package hostrunner

import (
	"testing"
	"time"
)

// obs helpers.
func systemLink() Observation {
	return Observation{Fresh: true, Phase: PhaseMenu, Connection: ConnSystemLink}
}

// hosting reports the create-game screens with a LIVE cursor parked at index 0 of
// both carousels (13 maps / 26 gametypes) — the common case where a default-Steps
// pick (target 0) lands immediately. Use hostingCur to place the cursor elsewhere.
func hosting() Observation { return hostingCur(0, 0) }

// hostingCur is a hosting observation with the map/gametype select-list cursors at
// the given live indices (counts 13/26, both valid) — for exercising the closed-
// loop carousel navigation.
func hostingCur(mapCur, gtCur int) Observation {
	return Observation{
		Fresh: true, Phase: PhaseMenu, Connection: ConnHosting,
		MapCursor: mapCur, MapCursorCount: 13, MapCursorValid: true,
		GametypeCursor: gtCur, GametypeCursorCount: 26, GametypeCursorValid: true,
	}
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

// TestSequenceNavigatesToChosenCard: a pick with a non-default target index drives
// the carousel toward it CLOSED-LOOP — pressing the D-pad only after the previous
// move has visibly landed (live cursor changed), and pressing A only once the
// re-read cursor == target — for both the map and gametype cards.
func TestSequenceNavigatesToChosenCard(t *testing.T) {
	sel := NewAtomicSelector()
	sel.Set(Pick{Name: "wizard", Steps: 2}, Pick{Name: "koth", Steps: 1})
	s := DefaultHostSequence(DefaultTiming, sel)
	t0 := time.Unix(1000, 0)

	s.Step(systemLink(), t0) // Y
	// Map card, target index 2. The cursor moves one card per confirmed press.
	if a := s.Step(hostingCur(0, 0), t0.Add(1*time.Second)); a.Key() != "Right" {
		t.Fatalf("map nav from 0 should press Right, got %v", a)
	}
	if a := s.Step(hostingCur(1, 0), t0.Add(2*time.Second)); a.Key() != "Right" {
		t.Fatalf("map nav from 1 should press Right, got %v", a)
	}
	if a := s.Step(hostingCur(2, 0), t0.Add(3*time.Second)); a.Key() != "a" || a.Intent != "select map" {
		t.Fatalf("cursor reached target 2 → press A (map), got %v", a)
	}
	// Gametype card after the blind timer, target index 1.
	if a := s.Step(hostingCur(2, 0), t0.Add(3*time.Second+DefaultTiming.BlindAdvanceAfter)); a.Key() != "Right" {
		t.Fatalf("gametype nav from 0 should press Right, got %v", a)
	}
	if a := s.Step(hostingCur(2, 1), t0.Add(6*time.Second)); a.Key() != "a" || a.Intent != "select gametype" {
		t.Fatalf("gametype cursor reached target 1 → press A, got %v", a)
	}
}

// TestSequenceCardWaitsForCursorMove: after a nav press the step HOLDS until the
// live cursor actually changes (no overshoot) — it must not press the D-pad again
// on the same cursor before RepressAfter.
func TestSequenceCardWaitsForCursorMove(t *testing.T) {
	sel := NewAtomicSelector()
	sel.Set(Pick{Name: "wizard", Steps: 5}, Pick{Name: "slayer"})
	s := DefaultHostSequence(DefaultTiming, sel)
	t0 := time.Unix(1000, 0)
	s.Step(systemLink(), t0) // Y

	if a := s.Step(hostingCur(0, 0), t0.Add(1*time.Second)); a.Key() != "Right" {
		t.Fatalf("first map nav should press Right, got %v", a)
	}
	// Same cursor, within RepressAfter → wait (don't double-press → don't overshoot).
	if a := s.Step(hostingCur(0, 0), t0.Add(1500*time.Millisecond)); a.Kind != ActionWait {
		t.Fatalf("nav should hold until cursor moves, got %v (%s)", a.Kind, a.Reason)
	}
}

// TestSequenceCardRepressesStuckCursor: if a press is dropped (cursor never moves),
// the step RE-PRESSES the D-pad after RepressAfter rather than hanging.
func TestSequenceCardRepressesStuckCursor(t *testing.T) {
	sel := NewAtomicSelector()
	sel.Set(Pick{Name: "wizard", Steps: 5}, Pick{Name: "slayer"})
	s := DefaultHostSequence(DefaultTiming, sel)
	t0 := time.Unix(1000, 0)
	s.Step(systemLink(), t0) // Y

	s.Step(hostingCur(0, 0), t0.Add(1*time.Second)) // Right
	a := s.Step(hostingCur(0, 0), t0.Add(1*time.Second+DefaultTiming.RepressAfter+time.Millisecond))
	if a.Key() != "Right" {
		t.Fatalf("stuck cursor should re-press Right after RepressAfter, got %v (%s)", a.Key(), a.Reason)
	}
}

// TestSequenceCardWrapsShortWay: from a high cursor toward a low target, the
// shorter way around the wrapping ring is BACKWARD (Left), not many Rights.
func TestSequenceCardWrapsShortWay(t *testing.T) {
	sel := NewAtomicSelector()
	sel.Set(Pick{Name: "battlecreek", Steps: 1}, Pick{Name: "slayer"}) // map target 1
	s := DefaultHostSequence(DefaultTiming, sel)
	t0 := time.Unix(1000, 0)
	s.Step(systemLink(), t0) // Y
	// Cursor at 12 of 13; target 1. Forward = 2 (12→13/0→1 via wrap), backward = 11.
	// 2 < 11, so the short way is FORWARD (Right, relying on wrap).
	if a := s.Step(hostingCur(12, 0), t0.Add(1*time.Second)); a.Key() != "Right" {
		t.Fatalf("cursor 12 → target 1 (count 13) should go Right (wrap, 2 steps), got %v", a)
	}
	// Now target 5 from cursor 1: forward = 4, backward = 9 → Right.
	sel.Set(Pick{Name: "x", Steps: 5}, Pick{Name: "slayer"})
	// And a case where backward is shorter: cursor 1, target 11 → fwd 10, bwd 3 → Left.
	sel.Set(Pick{Name: "y", Steps: 11}, Pick{Name: "slayer"})
	s2 := DefaultHostSequence(DefaultTiming, sel)
	s2.Step(systemLink(), t0)
	if a := s2.Step(hostingCur(1, 0), t0.Add(1*time.Second)); a.Key() != "Left" {
		t.Fatalf("cursor 1 → target 11 (count 13) should go Left (backward, 3 steps), got %v", a)
	}
}

// TestSequenceCardHoldsWithoutCursor: with NO live cursor the card step HOLDS
// (never presses A on a blind default) — the closed loop refuses to commit blind.
func TestSequenceCardHoldsWithoutCursor(t *testing.T) {
	sel := NewAtomicSelector()
	sel.Set(Pick{Name: "bloodgulch"}, Pick{Name: "slayer"})
	s := DefaultHostSequence(DefaultTiming, sel)
	t0 := time.Unix(1000, 0)
	s.Step(systemLink(), t0) // Y
	// Hosting but the widget isn't readable (no cursor) → hold, no A.
	noCursor := Observation{Fresh: true, Phase: PhaseMenu, Connection: ConnHosting}
	for i := 0; i < 4; i++ {
		a := s.Step(noCursor, t0.Add(time.Duration(1+i)*time.Second))
		if a.Kind == ActionTap {
			t.Fatalf("must not press without a live cursor (tick %d), got tap %q", i, a.Key())
		}
	}
}

// TestSequenceGametypeCustomPrefix: a gametype pick's index is in the ENUMERATION
// (built-in) space, but the live carousel prepends custom variants — so the runner
// must offset the target by the custom prefix (widgetCount − enumLen) and press A
// on the SHIFTED widget index, not the raw enumeration index.
func TestSequenceGametypeCustomPrefix(t *testing.T) {
	sel := NewAtomicSelector()
	// Map default (0), gametype "Race" at enumeration index 15.
	sel.Set(Pick{Name: "bloodgulch", Steps: 0}, Pick{Name: "Race", Steps: 15})
	s := DefaultHostSequence(DefaultTiming, sel)
	t0 := time.Unix(1000, 0)

	// hostGT: hosting obs with map cursor 0 (default) and the given gametype cursor,
	// gametype list len 26 built-ins over a 27-card carousel (1 custom prepended).
	hostGT := func(gtCur int) Observation {
		o := hostingCur(0, gtCur)
		o.GametypeCursorCount = 27
		o.GametypeListLen = 26
		return o
	}

	s.Step(systemLink(), t0)                      // Y
	s.Step(hostingCur(0, 0), t0.Add(time.Second)) // map cursor 0 == target 0 → A (map)
	// Advance past the map card's blind timer into select-gametype.
	adv := t0.Add(time.Second + DefaultTiming.BlindAdvanceAfter)
	// Drive the gametype cursor up to the SHIFTED target 16 (= ustr 15 + prefix 1).
	// From cursor 14: carouselNav(14,16,27) → Right; at 16 → A.
	if a := s.Step(hostGT(14), adv); a.Key() != "Right" {
		t.Fatalf("gametype nav from 14 toward 16 should press Right, got %v", a)
	}
	if a := s.Step(hostGT(15), adv.Add(time.Second)); a.Key() != "Right" {
		t.Fatalf("gametype nav from 15 toward 16 should press Right, got %v", a)
	}
	a := s.Step(hostGT(16), adv.Add(2*time.Second))
	if a.Key() != "a" || a.Intent != "select gametype" {
		t.Fatalf("gametype cursor at shifted target 16 (ustr 15 + prefix 1) should press A, got %v", a)
	}
}

// TestGametypeCustomPrefix locks the prefix computation.
func TestGametypeCustomPrefix(t *testing.T) {
	cases := []struct {
		count, listLen, want int
	}{
		{27, 26, 1}, // one custom variant prepended (the live-verified case)
		{26, 26, 0}, // stock disc, no customs
		{29, 26, 3}, // three customs
		{26, 0, 0},  // enumeration not available → no shift (fail safe)
		{20, 26, 0}, // widget < enum (shouldn't happen) → clamp to 0, never negative
	}
	for _, c := range cases {
		got := gametypeCustomPrefix(Observation{GametypeCursorCount: c.count, GametypeListLen: c.listLen})
		if got != c.want {
			t.Errorf("gametypeCustomPrefix(count=%d,len=%d) = %d, want %d", c.count, c.listLen, got, c.want)
		}
	}
}

// TestCarouselNav locks the pure shorter-path decision incl. wrap.
func TestCarouselNav(t *testing.T) {
	cases := []struct {
		cursor, target, count int
		wantRight             bool
		wantRemaining         int
	}{
		{0, 0, 13, true, 0},   // already on target
		{0, 2, 13, true, 2},   // forward, no wrap
		{5, 3, 13, false, 2},  // backward shorter, no wrap
		{12, 1, 13, true, 2},  // forward via wrap (2) beats backward (11)
		{1, 11, 13, false, 3}, // backward (3) beats forward via wrap (10)
		{16, 18, 27, true, 2}, // gametype forward (doc: Race 16 → CTF 18)
		{0, 0, 0, true, 0},    // empty list guard
		{6, 6, 13, true, 0},   // on target mid-list
		{0, 12, 13, false, 1}, // target just below via one Left (wrap)
	}
	for _, c := range cases {
		right, rem := carouselNav(c.cursor, c.target, c.count)
		if right != c.wantRight || rem != c.wantRemaining {
			t.Errorf("carouselNav(%d,%d,%d) = (right=%v, rem=%d), want (right=%v, rem=%d)",
				c.cursor, c.target, c.count, right, rem, c.wantRight, c.wantRemaining)
		}
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

// TestNavDrivesFromMainMenu: the nav phase drives the front-end menu toward
// System Link instead of blocking. THE fix — before it, the runner stalled
// forever on the main menu because step 1 (create-game) required
// game_connection==1 with nothing to bridge the box from the menu.
func TestNavDrivesFromMainMenu(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	a := s.Step(mainMenu(), time.Unix(1000, 0))
	if a.Kind != ActionTap || a.Key() != "up" {
		t.Fatalf("on the main menu the nav phase must drive (tap %q), got %v (%s)", "up", a.Kind, a.Reason)
	}
}

// TestSequenceBlocksOnWrongScreen: a step never presses blindly on an
// unexpected screen. The nav step blocks when the front-end bracket is lost
// before System Link is reached (a non-menu, non-system-link read).
func TestSequenceBlocksOnWrongScreen(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	// Fresh but neither the front-end menu (MenuActive=false ⇒ ScreenUnknown)
	// nor System Link: the nav step can't confirm progress and isn't on its
	// entry bracket, so it blocks rather than pressing on.
	wrong := Observation{Fresh: true, Phase: PhaseMenu, MenuActive: false, Connection: ConnMenu}
	a := s.Step(wrong, time.Unix(1000, 0))
	if a.Kind != ActionBlocked {
		t.Fatalf("got %v (%s), want blocked on wrong screen", a.Kind, a.Reason)
	}
}

// TestNavPhaseFullChain: the nav macro emits its fixed key sequence one press
// per NavKeyInterval while at the front-end menu, then re-emits the last key
// (the SELECT PROFILE confirms) until game_connection flips to 1 — at which
// point it completes and create-game presses Y. Mirrors the live rig chain
// main-menu(0) → System Link(1) → hosting(2).
func TestNavPhaseFullChain(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	t0 := time.Unix(1000, 0)
	navKeys := []string{"up", "up", "down", "a", "up", "up", "up", "down", "down", "a"}

	// Walk the fixed sequence: each press is gated by NavKeyInterval and, while
	// still at the front-end menu (game_connection stays 0 across the whole nav),
	// emits the next key.
	now := t0
	for i, want := range navKeys {
		a := s.Step(mainMenu(), now)
		if a.Kind != ActionTap || a.Key() != want {
			t.Fatalf("nav key %d: got %v (%s), want tap %q", i, a.Kind, a.Reason, want)
		}
		now = now.Add(DefaultTiming.NavKeyInterval)
	}
	// Keys exhausted but still on the menu → re-emit the last key ("a") to walk
	// the profile join/pick/all-ready confirms.
	a := s.Step(mainMenu(), now)
	if a.Kind != ActionTap || a.Key() != "a" {
		t.Fatalf("post-sequence: want re-emit of last key \"a\", got %v (%s)", a.Kind, a.Reason)
	}
	now = now.Add(DefaultTiming.NavKeyInterval)

	// game_connection flips to 1 (System Link browser): nav completes and
	// create-game fires Y.
	a = s.Step(systemLink(), now)
	if a.Kind != ActionTap || a.Key() != "y" {
		t.Fatalf("on System Link, create-game must press Y, got %v (%s)", a.Kind, a.Reason)
	}
}

// TestNavPhaseSkippedWhenAlreadySystemLink: a box already on System Link skips
// the nav phase entirely (catch-up) and goes straight to create-game.
func TestNavPhaseSkippedWhenAlreadySystemLink(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	a := s.Step(systemLink(), time.Unix(1000, 0))
	if a.Kind != ActionTap || a.Key() != "y" {
		t.Fatalf("already on System Link: want create-game Y (nav skipped), got %v (%s)", a.Kind, a.Reason)
	}
}

// TestNavPhaseBlocksWithoutNetwork: if the box can never reach System Link
// (e.g. Halo refuses with no active network) the nav step exhausts its retry
// budget and blocks — surfacing the real precondition instead of pressing A
// forever.
func TestNavPhaseBlocksWithoutNetwork(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	now := time.Unix(1000, 0)
	blocked := false
	for i := 0; i < len(mainMenuNavKeys())+navRetryBudget+2; i++ {
		a := s.Step(mainMenu(), now) // stuck at the menu, game_connection never → 1
		now = now.Add(DefaultTiming.NavKeyInterval)
		if a.Kind == ActionBlocked {
			blocked = true
			break
		}
	}
	if !blocked {
		t.Fatal("nav step should block after exhausting its retry budget when System Link is unreachable")
	}
}

func mainMenuNavKeys() []string {
	return []string{"up", "up", "down", "a", "up", "up", "up", "down", "down", "a"}
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
