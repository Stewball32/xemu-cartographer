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
// systemLink is the System Link GAMES BROWSER — now identified by its high-GVA
// widget (MenuItemSystemLinkGames), NOT game_connection (which reads stale here).
// Connection is left ConnMenu (stale-0) on purpose, so the tests prove the runner
// creates off the widget regardless of the flaky conn read.
func systemLink() Observation {
	return Observation{Fresh: true, Phase: PhaseMenu, MenuActive: true,
		Connection: ConnMenu, MenuItem: MenuItemSystemLinkGames}
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

// TestSelectMapKeepsDrivingMidScroll reproduces Stewart's f8513e6 "one Right press
// then hangs" bug: after the first NavKey press the carousel scrolls, so for a tick
// the list is still UP (MapCursorCount>0) but the index reads out-of-range
// (MapCursorValid=false). The created lobby's DEFAULT map/gametype are already
// resident (Y-create runs before the pick). The select-map step must KEEP DRIVING
// toward the target — never treat the mid-scroll tick + resident defaults as "past
// this card" and advance to reach-lobby.
func TestSelectMapKeepsDrivingMidScroll(t *testing.T) {
	sel := FixedSelector{Map: Pick{Name: "Temple", Steps: 7}, Gametype: Pick{Name: "Slayer", Steps: 3}}
	s := DefaultHostSequence(DefaultTiming, sel)
	t0 := time.Unix(1000, 0)

	s.Step(systemLink(), t0)                           // → create-game presses Y
	a := s.Step(hostingCur(0, 0), t0.Add(time.Second)) // → select-map drives toward 7
	if a.Kind != ActionTap || (a.Key() != "Right" && a.Key() != "Left") {
		t.Fatalf("expected a nav drive toward target 7, got %v key=%q", a.Kind, a.Key())
	}

	// Mid-scroll: list still up (count>0), index out of range (Valid=false), and the
	// lobby's default map/gametype are resident. Must HOLD, not advance.
	scrolling := Observation{
		Fresh: true, Phase: PhaseMenu, Connection: ConnHosting,
		MapCursorCount: 13, MapCursorValid: false,
		Map: "bloodgulch", Gametype: "slayer",
	}
	a = s.Step(scrolling, t0.Add(1200*time.Millisecond))
	if a.Kind == ActionDone {
		t.Fatalf("select-map completed mid-scroll — must keep driving (done: %s)", a.Reason)
	}
	if cur, _ := s.Current(); cur.Name != "select-map" {
		t.Fatalf("sequence left select-map mid-scroll (now %q) — the one-press bug", cur.Name)
	}

	// Scroll settles at the target: cursor==7 → commit with A.
	a = s.Step(hostingCur(7, 0), t0.Add(1500*time.Millisecond))
	if a.Kind != ActionTap || a.Key() != "a" {
		t.Fatalf("expected A to commit at target, got %v key=%q", a.Kind, a.Key())
	}
}

// TestColdMainMenuPrime reproduces the cold-boot main-menu hang and asserts the
// A→(wait tree)→B prime: main_menu=1 but the CE widget tree isn't built (dela empty,
// menu_focus 0). A direction press doesn't build it (looped on Down/b); pressing A
// into a reversible submenu does. The planner primes with A, waits for the tree, then
// B, then resumes normal nav. A genuinely off-route Unknown (dela populated) still Backs.
func TestColdMainMenuPrime(t *testing.T) {
	t0 := time.Unix(1000, 0)
	cold := Observation{Fresh: true, Phase: PhaseMenu, MenuActive: true, Connection: ConnMenu,
		MenuItem: MenuItemUnknown, Dela: "", MenuFocus: 0}
	s := DefaultHostSequence(DefaultTiming, proceedSelector())

	// 1. Cold blank main menu → press A to open a submenu (build the tree), NOT Down/B.
	if a := s.Step(cold, t0); a.Kind != ActionTap || a.Key() != "a" {
		t.Fatalf("cold menu: got %v key=%q, want tap a (prime open submenu)", a.Kind, a.Key())
	}
	// 2. A landed → on a submenu now (dela populated / menu_focus set) → press B to
	//    return (after the NavKeyInterval pacing window).
	sub := Observation{Fresh: true, Phase: PhaseMenu, MenuActive: true, Connection: ConnMenu,
		MenuItem: MenuItemUnknown, Dela: `ui\shell\main_menu\load_game_menu\foo`, MenuFocus: 0x80046578}
	s.Step(sub, t0.Add(300*time.Millisecond)) // confirm A landed (focus changed 0→set)
	if a := s.Step(sub, t0.Add(1400*time.Millisecond)); a.Kind != ActionTap || a.Key() != "b" {
		t.Fatalf("tree built on submenu → got %v key=%q (%s), want tap b (prime return)", a.Kind, a.Key(), a.Reason)
	}
	// 3. B landed → back on the main menu with the tree built (dela reads an item) →
	//    NORMAL nav resumes (MULTIPLAYER highlighted → A).
	built := Observation{Fresh: true, Phase: PhaseMenu, MenuActive: true, Connection: ConnMenu,
		MenuItem: MenuItemMultiplayer, Dela: `ui\shell\main_menu\main_menu_item_multiplayer`, MenuFocus: 0x80046600}
	s.Step(built, t0.Add(1700*time.Millisecond)) // confirm B landed
	if a := s.Step(built, t0.Add(2900*time.Millisecond)); a.Kind != ActionTap || a.Key() != "a" {
		t.Fatalf("back on built menu (MULTIPLAYER) → got %v key=%q (%s), want tap a (enter MP)", a.Kind, a.Key(), a.Reason)
	}

	// Off-route Unknown with a POPULATED highlight (dela set, menu_focus set) must NOT
	// prime — it Back-normalises.
	off := Observation{Fresh: true, Phase: PhaseMenu, MenuActive: true, Connection: ConnMenu,
		MenuItem: MenuItemUnknown, Dela: `ui\shell\main_menu\settings_select\player_setup\foo`, MenuFocus: 0x80046700}
	s2 := DefaultHostSequence(DefaultTiming, proceedSelector())
	if a := s2.Step(off, t0); a.Kind != ActionTap || a.Key() != "b" {
		t.Fatalf("off-route Unknown: got %v key=%q, want tap b (Back-normalise, no prime)", a.Kind, a.Key())
	}
}

// The prime must NEVER fire outside the cold main menu — not on SELECT MAP (a live
// lobby screen) even with a blank-ish read.
func TestPrimeNeverFiresOffMainMenu(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	// Hosting (ConnHosting) with no readable highlight is NOT the cold main menu.
	o := Observation{Fresh: true, Phase: PhaseMenu, MenuActive: true, Connection: ConnHosting,
		MenuItem: MenuItemUnknown, Dela: "", MenuFocus: 0}
	a := s.Step(o, time.Unix(1000, 0))
	if a.Kind == ActionTap && a.Key() == "a" {
		t.Fatalf("prime must not fire off the main menu (ConnHosting), got tap a")
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
	// Dela non-empty: a screen with a READABLE menu state has a populated highlight
	// widget (mapped or, off-route, unmapped-but-present) — only a cold un-woken main
	// menu reads dela empty (that case is constructed explicitly in
	// TestColdMainMenuWakesWithDown, not via this helper).
	return Observation{Fresh: true, Phase: PhaseMenu, MenuActive: true, Connection: ConnMenu, MenuItem: item, MenuFocus: focus, Dela: `ui\shell\main_menu\_hl`}
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

// The real System Link flow (RE'd from a networked box): enter System Link (A on
// the conn item) → SELECT PROFILE at game_connection==0 (join/select/confirm = A
// presses, screen unmapped: MenuItem Unknown or Profile) → the server_list games
// browser at conn==1, where create-game presses Y (CREATE). The host must never
// press A on the games browser (that JOINS) — Y is keyed strictly to conn==1, and
// A advances only the conn==0 entry screens.
func TestNavSystemLinkFlow(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	now := time.Unix(1000, 0)
	var focus uint32 = 0x8000
	tick := func(o Observation) Action {
		now = now.Add(DefaultTiming.NavKeyInterval + time.Millisecond)
		return s.Step(o, now)
	}
	// 1. On the System Link item → press A ONCE (starts the connect-hold).
	if a := tick(obsMI(MenuItemSystemLink, focus)); a.Kind != ActionTap || a.Key() != "a" {
		t.Fatalf("System Link item → A once, got %v key=%q (%s)", a.Kind, a.Key(), a.Reason)
	}
	// 2. While connecting (same item, conn still menu, highlight unchanged) the
	//    planner HOLDS — no re-press — across many ticks (the ~6s link-up).
	for i := 0; i < 5; i++ {
		if a := tick(obsMI(MenuItemSystemLink, focus)); a.Kind == ActionTap {
			t.Fatalf("must NOT re-press while System Link connects (tick %d), got tap %q", i, a.Key())
		}
	}
	// 3. Screen moves to SELECT PROFILE (conn=0, unmapped): connect-hold clears
	//    (a wait), then the profile flow A-advances.
	focus++
	if a := tick(obsMI(MenuItemUnknown, focus)); a.Kind == ActionTap {
		t.Fatalf("first tick on the moved screen clears connecting (wait), got tap %q", a.Key())
	}
	if a := tick(obsMI(MenuItemUnknown, focus)); a.Kind != ActionTap || a.Key() != "a" {
		t.Fatalf("Select Profile → advance with A, got %v key=%q (%s)", a.Kind, a.Key(), a.Reason)
	}
	// 4. server_list games browser via the HIGH-GVA widget (MenuItemSystemLinkGames),
	//    with game_connection STALE-0 (obsMI sets Connection:ConnMenu) — the runner
	//    must still CREATE with Y off the widget, never A (JOIN). This is the exact
	//    conn-staleness bug: the old "Y at conn==1" would have pressed A here.
	if a := tick(obsMI(MenuItemSystemLinkGames, focus)); a.Kind != ActionTap || a.Key() != "y" {
		t.Fatalf("server_list widget (conn stale-0) → CREATE with Y off the widget, got %v key=%q (%s)", a.Kind, a.Key(), a.Reason)
	}
}

// The System Link games browser is recognised off its high-GVA widget path, so the
// host CREATES with Y even when the stale-prone game_connection low global reads 0
// (ConnMenu) — never A (which would JOIN). Direct Classify/decision assertion.
func TestSystemLinkGamesCreatesOffWidgetDespiteStaleConn(t *testing.T) {
	obs := Observation{Fresh: true, Phase: PhaseMenu, MenuActive: true,
		Connection: ConnMenu, MenuItem: MenuItemSystemLinkGames} // conn STALE-0
	if got := Classify(obs); got != ScreenSystemLink {
		t.Fatalf("server_list widget must classify ScreenSystemLink off the widget (conn stale-0), got %s", got)
	}
}

// The System Link connect-hold re-presses ONCE if the single A genuinely dropped —
// after the generous window elapses with no screen/conn movement (not every tick).
func TestNavSystemLinkConnectRepressAfterTimeout(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	now := time.Unix(1000, 0)
	var focus uint32 = 0x8000
	// Press A on the System Link item.
	now = now.Add(DefaultTiming.NavKeyInterval + time.Millisecond)
	if a := s.Step(obsMI(MenuItemSystemLink, focus), now); a.Key() != "a" {
		t.Fatalf("System Link item → A, got %q", a.Key())
	}
	// Holds (no re-press) up to the timeout.
	now = now.Add(sysLinkConnectTimeout - time.Second)
	if a := s.Step(obsMI(MenuItemSystemLink, focus), now); a.Kind == ActionTap {
		t.Fatalf("must still hold before the connect timeout, got tap %q", a.Key())
	}
	// Past the timeout with no movement → re-press once.
	now = now.Add(2 * time.Second)
	if a := s.Step(obsMI(MenuItemSystemLink, focus), now); a.Kind != ActionTap || a.Key() != "a" {
		t.Fatalf("after the connect timeout a dropped press should re-fire A once, got %v key=%q", a.Kind, a.Key())
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

// SELECT PROFILE must A-advance BRISKLY: each A is confirmed by an observed
// focus change, so the next one goes out on the SHORT ProfileAdvanceInterval —
// not the conservative NavKeyInterval settle window that made it crawl. It must
// still never press while the previous press hasn't landed (no overshoot), and
// the System Link CONNECT hold must stay untouched.
func TestSelectProfileAdvancesFast(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	now := time.Unix(1000, 0)
	var focus uint32 = 0x8000
	step := func(o Observation, d time.Duration) Action {
		now = now.Add(d)
		return s.Step(o, now)
	}

	// Enter System Link (starts the connect-hold), then the screen moves to SELECT
	// PROFILE — the highlight leaves the conn item, clearing the hold.
	if a := step(obsMI(MenuItemSystemLink, focus), DefaultTiming.NavKeyInterval+time.Millisecond); a.Key() != "a" {
		t.Fatalf("expected the single A on the System Link item, got %v", a)
	}
	focus++
	step(obsMI(MenuItemProfile, focus), 500*time.Millisecond) // connect-hold clears

	// First profile A.
	a := step(obsMI(MenuItemProfile, focus), DefaultTiming.ProfileAdvanceInterval+time.Millisecond)
	if a.Kind != ActionTap || a.Key() != "a" {
		t.Fatalf("Select Profile → A, got %v key=%q (%s)", a.Kind, a.Key(), a.Reason)
	}

	// NOT landed yet (focus unchanged) → must HOLD, never press again (no overshoot),
	// even well past the short profile interval.
	if a := step(obsMI(MenuItemProfile, focus), DefaultTiming.ProfileAdvanceInterval+time.Millisecond); a.Kind == ActionTap {
		t.Fatalf("must not re-press before the A lands (overshoot), got tap %q", a.Key())
	}

	// Landed (focus changed) → the NEXT A goes out on the SHORT interval. A tick
	// this soon would still be inside NavKeyInterval, so a tap here proves the
	// profile-specific pacing (and that the landing tick isn't wasted).
	focus++
	next := step(obsMI(MenuItemProfile, focus), DefaultTiming.ProfileAdvanceInterval+time.Millisecond)
	if next.Kind != ActionTap || next.Key() != "a" {
		t.Fatalf("after landing, the next A should fire on the short interval, got %v key=%q (%s)",
			next.Kind, next.Key(), next.Reason)
	}
	if DefaultTiming.ProfileAdvanceInterval >= DefaultTiming.NavKeyInterval {
		t.Fatal("ProfileAdvanceInterval must be shorter than NavKeyInterval to be a speed-up")
	}
}

// SHORTEST PATH WITH WRAPAROUND, proven for the raw ring math: the carousels LOOP,
// so reaching a target must take the shorter way round — LEFT (wrap backward) when
// that's closer, not all the way right.
func TestCarouselWrapsTheShortWay(t *testing.T) {
	const n = 36
	cases := []struct {
		cursor, target int
		wantRight      bool
		wantRemaining  int
		why            string
	}{
		{0, 34, false, 2, "0→34 wraps BACKWARD in 2 (not 34 forward)"},
		{34, 0, true, 2, "34→0 wraps FORWARD in 2 (not 34 backward)"},
		{0, 5, true, 5, "0→5 is forward"},
		{5, 0, false, 5, "5→0 is backward"},
		{2, 30, false, 8, "2→30 wraps backward in 8 (not 28 forward)"},
		{30, 2, true, 8, "30→2 wraps forward in 8"},
		{7, 7, true, 0, "already on target"},
		{0, 18, true, 18, "exact half → forward (tie)"},
	}
	for _, c := range cases {
		right, rem := carouselNav(c.cursor, c.target, n)
		if right != c.wantRight || rem != c.wantRemaining {
			t.Errorf("carouselNav(%d,%d,%d) = right:%v rem:%d, want right:%v rem:%d — %s",
				c.cursor, c.target, n, right, rem, c.wantRight, c.wantRemaining, c.why)
		}
		// The chosen route is never longer than the other way round.
		if other := n - rem; rem > 0 && rem > other {
			t.Errorf("carouselNav(%d,%d,%d) took the LONG way (%d > %d)", c.cursor, c.target, n, rem, other)
		}
	}
	// Exhaustive: every (cursor,target) pair picks the true minimum.
	for cur := 0; cur < n; cur++ {
		for tgt := 0; tgt < n; tgt++ {
			fwd := ((tgt-cur)%n + n) % n
			want := fwd
			if n-fwd < fwd {
				want = n - fwd
			}
			if _, rem := carouselNav(cur, tgt, n); rem != want {
				t.Fatalf("carouselNav(%d,%d,%d) rem=%d, want min-steps %d", cur, tgt, n, rem, want)
			}
		}
	}
}

// The wrap-aware drive applies to BOTH carousels end-to-end: a high map target from
// cursor 0 presses LEFT, and so does a high gametype target.
func TestBothCardStepsWrapBackward(t *testing.T) {
	sel := NewAtomicSelector()
	// Map target 12 of 13 (1 back), gametype target 25 of 26 (1 back).
	sel.Set(Pick{Name: "lastmap", Steps: 12}, Pick{Name: "lastgt", Steps: 25})
	s := DefaultHostSequence(DefaultTiming, sel)
	t0 := time.Unix(1000, 0)

	s.Step(systemLink(), t0) // Y (create)
	if a := s.Step(hostingCur(0, 0), t0.Add(time.Second)); a.Key() != "Left" {
		t.Fatalf("map 0→12 of 13 should wrap LEFT (1 step), got %v key=%q (%s)", a.Kind, a.Key(), a.Reason)
	}
	// Cursor wrapped to the target → commit, advance to the gametype card.
	s.Step(hostingCur(12, 0), t0.Add(2*time.Second))                                    // A (commit map)
	a := s.Step(hostingCur(12, 0), t0.Add(2*time.Second+DefaultTiming.BlindAdvanceAfter)) // → gametype step
	if a.Key() != "Left" {
		t.Fatalf("gametype 0→25 of 26 should wrap LEFT (1 step), got %v key=%q (%s)", a.Kind, a.Key(), a.Reason)
	}
}

// REGRESSION (urgent, be052c6): the cold-menu prime must press A EXACTLY ONCE and then
// CONFIRM the widget tree before any further press. The cold menu's default highlight
// is SINGLE PLAYER and menu_focus stays 0 until the tree builds, so the ordinary
// RepressAfter path re-pressed A every 700ms — mashing it into Single Player and
// LAUNCHING A CAMPAIGN GAME.
func TestColdPrimePressesAOnlyOnce(t *testing.T) {
	cold := Observation{Fresh: true, Phase: PhaseMenu, MenuActive: true, Connection: ConnMenu,
		MenuItem: MenuItemUnknown, Dela: "", MenuFocus: 0}
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	t0 := time.Unix(1000, 0)

	if a := s.Step(cold, t0); a.Kind != ActionTap || a.Key() != "a" {
		t.Fatalf("prime should press A once, got %v key=%q", a.Kind, a.Key())
	}
	// Nothing changed (cold box, tree not built). For the WHOLE confirm window there
	// must be NO further press — most importantly none at RepressAfter (700ms).
	for _, d := range []time.Duration{
		DefaultTiming.RepressAfter + time.Millisecond, // the exact regression point
		DefaultTiming.RepressAfter * 2,
		primeConfirmWindow - 100*time.Millisecond,
	} {
		if a := s.Step(cold, t0.Add(d)); a.Kind == ActionTap {
			t.Fatalf("at +%v the prime pressed %q again — this mashes A into Single Player "+
				"and launches a campaign", d, a.Key())
		}
	}
	// Past the generous window a SECOND prime A is allowed (the first genuinely dropped)…
	a := s.Step(cold, t0.Add(primeConfirmWindow+time.Millisecond))
	if a.Kind != ActionTap || a.Key() != "a" {
		t.Fatalf("after the confirm window a retry A is allowed, got %v key=%q", a.Kind, a.Key())
	}
	// …but it is BOUNDED: eventually it blocks instead of pressing A forever.
	now := t0.Add(primeConfirmWindow + time.Millisecond)
	sawBlocked := false
	for i := 0; i < 6; i++ {
		now = now.Add(primeConfirmWindow + time.Millisecond)
		if s.Step(cold, now).Kind == ActionBlocked {
			sawBlocked = true
			break
		}
	}
	if !sawBlocked {
		t.Fatal("prime must BLOCK after primeMaxAttempts, never keep pressing A on the cold menu")
	}
}

// When the A DOES land (tree builds → dela/menu_focus populate) the prime backs out
// with B immediately — the fast path is unaffected by the safety hold.
func TestColdPrimeBacksOutAsSoonAsTreeBuilds(t *testing.T) {
	s := DefaultHostSequence(DefaultTiming, proceedSelector())
	t0 := time.Unix(1000, 0)
	cold := Observation{Fresh: true, Phase: PhaseMenu, MenuActive: true, Connection: ConnMenu,
		MenuItem: MenuItemUnknown, Dela: "", MenuFocus: 0}
	s.Step(cold, t0) // prime A

	built := Observation{Fresh: true, Phase: PhaseMenu, MenuActive: true, Connection: ConnMenu,
		MenuItem: MenuItemUnknown, Dela: `ui\shell\main_menu\campaign_select\foo`, MenuFocus: 0x80046578}
	// The landing tick ITSELF emits the B (no wasted tick) — well inside the safety
	// hold, proving the hold only slows the FAILURE path, never the working one.
	if a := s.Step(built, t0.Add(500*time.Millisecond)); a.Kind != ActionTap || a.Key() != "b" {
		t.Fatalf("tree built → back out with B promptly, got %v key=%q (%s)", a.Kind, a.Key(), a.Reason)
	}
}
