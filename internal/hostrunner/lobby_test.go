package hostrunner

import (
	"testing"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/vncinput"
)

// TestNavKeysAreVncinputLabels guards the ENTIRE host-flow key vocabulary against
// the shared input layer: every label the runner emits must be a member of
// vncinput.KEYSYM, or the vncinput pump rejects the press ("unknown key ...") and
// nothing reaches the box. This is the exact bug that shipped once — planNavKey
// returned lowercase "down" while vncinput's d-pad label is "Down" — masked
// because the rig's own /input RFB client was case-lenient. It covers planNavKey
// across every MenuItem, and the Key/NavKey/NavKeyBack on the default sequence.
func TestNavKeysAreVncinputLabels(t *testing.T) {
	valid := map[string]bool{}
	for _, l := range vncinput.SupportedLabels() {
		valid[l] = true
	}
	check := func(where, key string) {
		if key == "" {
			return
		}
		if !valid[key] {
			t.Errorf("%s emits %q, which is NOT a vncinput label — the pump would reject it (SupportedLabels: %v)", where, key, vncinput.SupportedLabels())
		}
	}
	// planNavKey over every classified item (incl. Unknown/off-route).
	for _, it := range []MenuItem{
		MenuItemUnknown, MenuItemMainOther, MenuItemMultiplayer,
		MenuItemSubmenuOther, MenuItemSystemLink, MenuItemProfile,
	} {
		check("planNavKey("+it.String()+")", planNavKey(it))
	}
	// The stuck-detection recovery key, and the card/select steps' labels.
	check("stuck-recovery", "b")
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	for _, st := range s.steps {
		check("step "+st.Name+".Key", st.Key)
		check("step "+st.Name+".NavKey", st.NavKey)
		check("step "+st.Name+".NavKeyBack", st.NavKeyBack)
	}
}

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

// obsMI builds a front-end observation with a given highlighted MenuItem and
// menu-focus pointer — the inputs the state-aware planner routes on.
func obsMI(item MenuItem, focus uint32) Observation {
	return Observation{Fresh: true, Phase: PhaseMenu, MenuActive: true, Connection: ConnMenu, MenuItem: item, MenuFocus: focus}
}

// TestNavDrivesFromMainMenu: on the main menu with a non-Multiplayer item
// highlighted, the state-aware planner steps toward MULTIPLAYER (Down) — THE fix.
func TestNavDrivesFromMainMenu(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	a := s.Step(obsMI(MenuItemMainOther, 0x8000), time.Unix(1000, 0))
	if a.Kind != ActionTap || a.Key() != "Down" {
		t.Fatalf("main menu, non-Multiplayer item: want Down toward Multiplayer, got %v (%s)", a.Kind, a.Reason)
	}
}

// TestNavRoutesByIdentity: the planner routes to System Link by ITEM IDENTITY,
// choosing each press from the highlighted MenuItem (not a key count), each
// confirmed by MenuFocus changing: MainOther→Down, Multiplayer→A, SubmenuOther→
// Down, SystemLink→A, then game_connection→1 and create-game→Y. (Live on H1 Perf
// there is NO profile-confirm on this path — choosing System Link flips
// game_connection directly; the profile enum is handled by TestNavRecovers*.)
func TestNavRoutesByIdentity(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	now := time.Unix(1000, 0)
	var focus uint32 = 0x8000
	step := func(item MenuItem, wantKey string) {
		now = now.Add(DefaultTiming.NavKeyInterval + time.Millisecond)
		a := s.Step(obsMI(item, focus), now) // emit the planned key
		if a.Kind != ActionTap || a.Key() != wantKey {
			t.Fatalf("highlighted=%s: want tap %q, got %v (%s)", item, wantKey, a.Kind, a.Reason)
		}
		focus++ // press landed (highlight/screen changed)
		now = now.Add(10 * time.Millisecond)
		if a := s.Step(obsMI(item, focus), now); a.Kind != ActionWait {
			t.Fatalf("confirm after %s: want wait (landed), got %v (%s)", item, a.Kind, a.Reason)
		}
	}
	step(MenuItemMainOther, "Down")    // main menu, wrong item → toward Multiplayer
	step(MenuItemMultiplayer, "a")     // on MULTIPLAYER → enter submenu
	step(MenuItemSubmenuOther, "Down") // submenu, wrong item → toward System Link
	step(MenuItemSystemLink, "a")      // on SYSTEM LINK → enter (game_connection→1)
	// The A on System Link lands us in the browser (game_connection→1): nav
	// completes and create-game presses Y.
	now = now.Add(DefaultTiming.NavKeyInterval + time.Millisecond)
	if a := s.Step(systemLink(), now); a.Kind != ActionTap || a.Key() != "y" {
		t.Fatalf("on System Link, create-game must press Y, got %v (%s)", a.Kind, a.Reason)
	}
}

// After entering System Link (A on the submenu item) the runner lands on the games
// BROWSER — an unmapped front-end screen (MenuItem=Unknown, MenuActive, and
// game_connection NOT yet 1). There A = JOIN an existing lobby (fine on an empty
// LAN, wrong on a populated one); only Y CREATES a new one. The host box must
// always create its own, so the planner presses Y there, never A.
func TestNavCreatesWithYOnSystemLinkBrowser(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	now := time.Unix(1000, 0)
	var focus uint32 = 0x8000
	press := func(item MenuItem) Action {
		now = now.Add(DefaultTiming.NavKeyInterval + time.Millisecond)
		a := s.Step(obsMI(item, focus), now) // emit
		focus++                              // press landed
		now = now.Add(10 * time.Millisecond)
		s.Step(obsMI(item, focus), now) // confirm landed
		return a
	}
	if a := press(MenuItemSystemLink); a.Kind != ActionTap || a.Key() != "a" {
		t.Fatalf("entering System Link should press A, got %v (%s)", a.Kind, a.Reason)
	}
	// Games browser (Unknown, conn still menu) → CREATE with Y, not A.
	now = now.Add(DefaultTiming.NavKeyInterval + time.Millisecond)
	a := s.Step(obsMI(MenuItemUnknown, focus), now)
	if a.Kind != ActionTap || a.Key() != "y" {
		t.Fatalf("System Link games browser: host must CREATE with Y, got %v key=%q (%s)", a.Kind, a.Key(), a.Reason)
	}
}

// TestNavRecoversFromProfile: a profile screen is OFF the host-creation route
// (reached only via Settings › Player Setup or Single Player › pick profile);
// pressing A there dives deeper forever, so the planner must Back out of it.
func TestNavRecoversFromProfile(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	now := time.Unix(1000, 0)
	if a := s.Step(obsMI(MenuItemProfile, 0x8000), now); a.Kind != ActionTap || a.Key() != "b" {
		t.Fatalf("on a profile screen: want Back (b) to escape, got %v (%s)", a.Kind, a.Reason)
	}
}

// TestNavStuckDetectionForcesBack: if the highlighted item never changes for
// navStuckThreshold consecutive emitted presses (an off-route screen the
// classifier reads as a routable enum, so the planned key keeps firing without
// progress), the planner forces a Back to escape. Modelled with MainOther, whose
// planned key is Down: after navStuckThreshold identical presses it flips to B.
func TestNavStuckDetectionForcesBack(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	now := time.Unix(1000, 0)
	var focus uint32 = 0x8000
	// Feed the SAME item every emit, advancing focus so each press "lands" (so the
	// stall is item-not-changing, not a dropped press). Down for the first
	// navStuckThreshold, then B.
	for i := 0; i < navStuckThreshold+1; i++ {
		now = now.Add(DefaultTiming.NavKeyInterval + time.Millisecond)
		a := s.Step(obsMI(MenuItemMainOther, focus), now)
		if a.Kind != ActionTap {
			t.Fatalf("press %d: want tap, got %v (%s)", i, a.Kind, a.Reason)
		}
		wantKey := "Down"
		if i >= navStuckThreshold {
			wantKey = "b"
		}
		if a.Key() != wantKey {
			t.Fatalf("press %d: want %q (stuck→%d), got %q", i, wantKey, s.navStuckCount, a.Key())
		}
		focus++ // press landed; item stays MainOther → stuck accrues
		now = now.Add(10 * time.Millisecond)
		s.Step(obsMI(MenuItemMainOther, focus), now) // confirm landed
	}
}

// TestNavRecoversFromOffRoute: from an unrecognised screen (MenuItemUnknown —
// Settings / Single Player / an unreadable screen) the planner presses B to
// Back-normalise, and resumes routing once a known item reappears.
func TestNavRecoversFromOffRoute(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	now := time.Unix(1000, 0)
	// Off-route: Unknown → B.
	if a := s.Step(obsMI(MenuItemUnknown, 0x8000), now); a.Kind != ActionTap || a.Key() != "b" {
		t.Fatalf("off-route: want Back (b) to normalise, got %v (%s)", a.Kind, a.Reason)
	}
	// B landed (focus changed); a main-menu item now shows → confirm, then route.
	now = now.Add(10 * time.Millisecond)
	if a := s.Step(obsMI(MenuItemMainOther, 0x8001), now); a.Kind != ActionWait {
		t.Fatalf("after B lands: want wait, got %v (%s)", a.Kind, a.Reason)
	}
	now = now.Add(DefaultTiming.NavKeyInterval + time.Millisecond)
	if a := s.Step(obsMI(MenuItemMainOther, 0x8001), now); a.Kind != ActionTap || a.Key() != "Down" {
		t.Fatalf("recovered to main menu: want Down toward Multiplayer, got %v (%s)", a.Kind, a.Reason)
	}
}

// TestSequenceBlocksOnWrongScreen: a select that mis-fired into a map/campaign
// (MenuActive==0 while game_connection is still menu) blocks rather than pressing
// on.
func TestSequenceBlocksOnWrongScreen(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	wrong := Observation{Fresh: true, Phase: PhaseMenu, MenuActive: false, Connection: ConnMenu}
	a := s.Step(wrong, time.Unix(1000, 0))
	if a.Kind != ActionBlocked {
		t.Fatalf("got %v (%s), want blocked (left front-end into a map)", a.Kind, a.Reason)
	}
}

// TestNavConfirmsMoveBeforeAdvancing: a DROPPED press (MenuFocus unchanged) is
// re-emitted rather than advancing.
func TestNavConfirmsMoveBeforeAdvancing(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	now := time.Unix(1000, 0)
	if a := s.Step(obsMI(MenuItemMainOther, 0x8000), now); a.Key() != "Down" {
		t.Fatalf("want first tap Down, got %v", a)
	}
	now = now.Add(100 * time.Millisecond) // focus unchanged, before RepressAfter → wait
	if a := s.Step(obsMI(MenuItemMainOther, 0x8000), now); a.Kind != ActionWait {
		t.Fatalf("dropped press before RepressAfter should wait, got %v (%s)", a.Kind, a.Reason)
	}
	now = now.Add(DefaultTiming.RepressAfter) // still unchanged → re-emit down
	if a := s.Step(obsMI(MenuItemMainOther, 0x8000), now); a.Kind != ActionTap || a.Key() != "Down" {
		t.Fatalf("dropped d-pad press must re-emit down, got %v (%s)", a.Kind, a.Reason)
	}
}

// TestNavPhaseSkippedWhenAlreadySystemLink: a box already on System Link skips
// the nav phase (catch-up) and goes straight to create-game.
func TestNavPhaseSkippedWhenAlreadySystemLink(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	a := s.Step(systemLink(), time.Unix(1000, 0))
	if a.Kind != ActionTap || a.Key() != "y" {
		t.Fatalf("already on System Link: want create-game Y (nav skipped), got %v (%s)", a.Kind, a.Reason)
	}
}

// TestNavPhaseBlocksWhenStuck: if the box never navigates (MenuFocus never
// changes) the planner burns its press budget and blocks, surfacing the cause.
func TestNavPhaseBlocksWhenStuck(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	now := time.Unix(1000, 0)
	blocked := false
	for i := 0; i < 200; i++ {
		a := s.Step(obsMI(MenuItemMainOther, 0x8000), now) // stuck: focus never changes
		now = now.Add(DefaultTiming.RepressAfter + time.Millisecond)
		if a.Kind == ActionBlocked {
			blocked = true
			break
		}
	}
	if !blocked {
		t.Fatal("nav step should block once its press budget is spent on a box that never navigates")
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
