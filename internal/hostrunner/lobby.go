package hostrunner

import (
	"fmt"
	"time"
)

// Transition is one gated step of the host-lobby flow. The machine presses Key
// only while On(obs) holds ("am I on the right screen?"), then waits until
// Done(obs) confirms the press landed before advancing. Blind returns true for
// the map/gametype card screens that expose no distinguishing global — those
// advance on a timed fallback (Sequence.blindAdvance) instead of Done, but only
// after On confirmed we entered the blind segment.
type Transition struct {
	Name   string
	Intent string
	Key    string
	On     func(Observation) bool
	Done   func(Observation) bool
	Blind  bool

	// Card navigation (map/gametype select screens). When NavKey is set this is a
	// card step. It drives the carousel CURSOR-RELATIVE and CLOSED-LOOP: each tick
	// it reads the LIVE highlighted index via CursorFn, computes the shorter path to
	// the target card (StepsFn), presses NavKey (forward) / NavKeyBack (reverse)
	// toward it — one confirmed move at a time — and presses Key (A) only once the
	// re-read cursor == target. No fixed default: the target is the pick's absolute
	// carousel index and the start is non-deterministic. ParkUntilSelection makes
	// the step HOLD (the front-loaded lobby wait) until the player has actually
	// chosen — the runner creates the lobby with Y right away, then parks at
	// map-select rather than pressing a blind default.
	//
	// StepsFn resolves the TARGET absolute carousel index from the live selection.
	// CursorFn reads the live cursor (index, count, ok) for THIS list from the
	// observation; ok=false when the widget isn't readable (the step then holds
	// rather than navigating blind). NavKeyBack is the reverse D-pad label used when
	// the shorter way around the ring is backwards (empty ⇒ forward-only).
	NavKey             string
	NavKeyBack         string
	ParkUntilSelection bool
	StepsFn            func(Selector) int
	CursorFn           func(Observation) (index, count int, ok bool)
	// TargetName resolves the chosen pick's display NAME (for the decision log), so
	// the runner's carousel drive shows WHICH map/gametype it's honoring — the
	// visible proof a Set-game pick took, not just a target index.
	TargetName func(Selector) string

	// Keys, when non-empty, makes this a MACRO NAV step: a fixed ordered key
	// sequence driven between two readable checkpoints — On at entry, Done at
	// exit — for FRONT-END menu navigation (main menu → System Link) where the
	// intermediate screens (Multiplayer submenu, SELECT PROFILE) expose no
	// distinguishing memory global, only game_connection==0/menu at both ends.
	// This is the SAME "blind but bracketed" basis the card steps already use
	// (§observation.go): timed presses fenced by readable state, never a press
	// into the void. Two robustness properties:
	//   - the LAST key is RE-EMITTED on the timer after the fixed sequence until
	//     Done holds — so a screen needing an unknown number of identical
	//     confirms (SELECT PROFILE: join → pick profile → "all ready") is
	//     self-correcting and a dropped press just re-fires;
	//   - it completes the INSTANT Done holds (game_connection→1) and blocks if
	//     it leaves the On bracket or exhausts its retry budget, so a desync
	//     surfaces instead of walking into a wrong screen.
	// Entry assumption (documented): the box is at the top-level CE main menu
	// with the standard multiplayer menu layout; a box parked deeper would need
	// B-normalization first (future hardening).
	Keys []string

	// Planner, when true, makes this a STATE-AWARE navigation step: rather than a
	// fixed key list it reads Observation.MenuItem each tick and picks the next
	// press by item IDENTITY, routing to System Link from ANY starting screen and
	// Back-normalising off-route (see stepPlanNav). Supersedes Keys.
	Planner bool

	// PrefixFn returns how many live carousel cards PRECEDE the enumerated list
	// (i.e. the offset to add to a pick's enumeration index to reach its live
	// widget index). For SELECT GAMETYPE this is the count of user-saved custom
	// variants prepended ahead of the built-ins; nil (SELECT MAP) means the widget
	// index equals the enumeration index. See GametypeListLen on Observation.
	PrefixFn func(Observation) int
}

// StepTiming controls debounce + blind-advance behaviour. Zero value uses
// DefaultTiming.
type StepTiming struct {
	// RepressAfter re-emits a step's key if Done still isn't observed this long
	// after the last press (covers a dropped KeyEvent / a slow menu).
	RepressAfter time.Duration
	// BlindAdvanceAfter advances a Blind step this long after it became active
	// even without a Done confirmation.
	BlindAdvanceAfter time.Duration
	// NavKeyInterval paces the macro-nav step (Transition.Keys): the gap between
	// front-end menu presses. Larger than BlindAdvanceAfter because the SELECT
	// PROFILE sub-screens take ~1–1.5s to settle before the next press registers
	// (measured live on the rig).
	NavKeyInterval time.Duration
}

// DefaultTiming is tuned for CE's ~30Hz menus over VNC.
var DefaultTiming = StepTiming{
	RepressAfter:      1200 * time.Millisecond,
	BlindAdvanceAfter: 900 * time.Millisecond,
	NavKeyInterval:    1200 * time.Millisecond,
}

func (t StepTiming) orDefault() StepTiming {
	if t.RepressAfter == 0 {
		t.RepressAfter = DefaultTiming.RepressAfter
	}
	if t.BlindAdvanceAfter == 0 {
		t.BlindAdvanceAfter = DefaultTiming.BlindAdvanceAfter
	}
	if t.NavKeyInterval == 0 {
		t.NavKeyInterval = DefaultTiming.NavKeyInterval
	}
	return t
}

// navRetryBudget caps how many extra times the macro-nav step re-emits its last
// key (the SELECT PROFILE confirms) while waiting for Done. Enough for the
// join → pick → all-ready A-presses plus a couple of dropped-press retries;
// beyond it the step blocks so a genuine "can't reach System Link" (e.g. no
// active network → Halo refuses) surfaces instead of pressing A forever.
const navRetryBudget = 8

// Sequence walks an ordered list of gated Transitions. It is the "never blind"
// core: it advances only on confirmed observations (or a timed fallback for the
// genuinely-blind card screens), never by counting presses. Not safe for
// concurrent use — the runner drives it from one goroutine.
type Sequence struct {
	steps  []Transition
	timing StepTiming
	sel    Selector // selection source for park gate + card navigation (may be nil)

	cursor        int
	enteredAt     time.Time // when the current step became active
	lastPress     time.Time // last time the current step's key was emitted
	pressed       bool      // has the current step's confirm key (A) been pressed
	navDone       int       // D-pad nav presses emitted for the current card step
	lastNavCursor int       // live cursor value at the last nav press (for confirmed-move debounce)
	navFocus      uint32    // menu-focus ptr recorded at the macro-nav step's last press (change ⇒ move landed)
	navPresses    int       // TOTAL macro-nav presses emitted incl. re-presses (drop budget)
	lastKey       string    // last key the planner emitted (for wait-reason text)
	navStuckItem  MenuItem  // highlighted item at the last emitted nav press (stuck-detection)
	navStuckCount int       // consecutive emitted nav presses with an unchanged highlighted item
	// sysLinkEntered is set once the planner presses A on the SYSTEM LINK submenu
	// item (multiplayer_type_conn_item). It then A-advances the Select Profile entry
	// flow (game_connection==0, MenuItem Unknown/Profile) until conn→1 reaches the
	// server_list games browser, where create-game presses Y (CREATE). See
	// stepPlanNav — this is what stops the host from Back-ing out of Select Profile.
	sysLinkEntered bool
	// sysLinkConnectAt is when the single A was pressed on the SYSTEM LINK item; the
	// planner then HOLDS (no re-press) until the screen/conn moves or
	// sysLinkConnectTimeout elapses, so it doesn't hammer A through the ~6s connect.
	// Zero when not connecting. See stepPlanNav.
	sysLinkConnectAt time.Time
}

// NewSequence builds a Sequence over steps with the given timing. Set Selector
// separately (DefaultHostSequence does) to enable the park gate + card nav.
func NewSequence(steps []Transition, timing StepTiming) *Sequence {
	return &Sequence{steps: steps, timing: timing.orDefault()}
}

// SetSelector wires the selection source used by the park gate + card navigation.
func (s *Sequence) SetSelector(sel Selector) { s.sel = sel }

// DefaultHostSequence is the CE system-link host flow:
//
//	System Link (gc==1) --Y--> [PARK @ map-select until pick] --nav+A--> gametype
//	  --nav+A--> LOBBY
//
// Refinement (Stewart, 2026-07-10): Y advertises the joinable lobby immediately,
// so the runner front-loads it and then HOLDS at the map-select card screen until
// a map/gametype is actually chosen (ParkUntilSelection) — no blind default pick.
//
// Cursor-relative nav (2026-07-11): the card presses are no longer blind. Each
// select-map / select-gametype step reads the LIVE highlighted index from the CE
// menu widget system (Observation.MapCursor / .GametypeCursor, +0x4C), drives the
// carousel toward the pick's absolute index the shorter way around the ring, and
// presses A only once the re-read cursor == target — closing the loop on confirmed
// state from any non-deterministic start (incl. wrap). See stepCard / carouselNav.
// The steps are still bracketed by readable checkpoints (On requires we're hosting,
// terminal Done requires a readable lobby). Keys use vncinput labels: Y='y', A='a',
// D-pad Right='Right', Left='Left'.
func DefaultHostSequence(timing StepTiming, sel Selector) *Sequence {
	atFrontEnd := func(o Observation) bool { return Classify(o) == ScreenMainMenu }
	onSystemLink := func(o Observation) bool { return Classify(o) == ScreenSystemLink }
	// reachedSystemLink is the nav phase's Done: true once the box is on System
	// Link OR any later host-flow screen. game_connection==1 is transient (it
	// becomes 2 the moment create-game fires), so a plain onSystemLink Done can't
	// be caught-up past once we've advanced — this "at or beyond" test lets the
	// catch-up loop skip the nav step for a box already in hosting/lobby/in-game.
	reachedSystemLink := func(o Observation) bool {
		switch Classify(o) {
		case ScreenSystemLink, ScreenHosting, ScreenLobby:
			return true
		}
		return false
	}
	hosting := func(o Observation) bool {
		s := Classify(o)
		return s == ScreenHosting || s == ScreenLobby
	}
	inLobby := func(o Observation) bool { return Classify(o) == ScreenLobby }

	seq := NewSequence([]Transition{
		{
			// NAV PHASE — STATE-AWARE navigator (2026-08-06 → generalised 2026-08-07):
			// drive the CE front-end to System Link FROM ANY starting screen. The
			// runner no longer assumes it's handed a box already on System Link
			// (game_connection==1) — the definitive beta stall — nor even at the main
			// menu: it reads WHICH screen+item the box is on and routes there,
			// self-correcting.
			//
			// It reads Observation.MenuItem (the highlighted front-end item,
			// classified from the CE UI widget heap — see haloce/menustate.go) and
			// picks the next press by IDENTITY, not a wrap-prone key count:
			//   main-menu non-Multiplayer → Down (toward MULTIPLAYER)
			//   main-menu MULTIPLAYER     → A (enter submenu)
			//   submenu non-System-Link   → Down (toward SYSTEM LINK / "conn")
			//   submenu SYSTEM LINK       → A (→ SELECT PROFILE)
			//   SELECT PROFILE            → A (join / pick / all-ready), until conn→1
			//   UNKNOWN / off-route       → B (Back-normalise toward the main menu)
			// The UNKNOWN→B branch is the recovery: from Settings, a Single-Player
			// screen, or any submenu we don't recognise, B backs out until a known
			// item reappears, then routing resumes. Every press is CONFIRMED by the
			// menu-focus pointer (Observation.MenuFocus) changing; a dropped press is
			// re-emitted until it lands — so drops can't mis-land. See stepPlanNav.
			//
			// On=atFrontEnd(recognised item) and Done=reachedSystemLink bracket it;
			// a select that mis-fired into a map/campaign (MenuActive→0) blocks
			// loudly instead of proceeding.
			Name: "nav-system-link", Intent: "→ system link (state-aware)",
			Planner: true,
			On:      atFrontEnd,
			Done:    reachedSystemLink,
		},
		{
			Name: "create-game", Intent: "create system-link game", Key: "y",
			On:   onSystemLink,
			Done: hosting, // landed in the hosting setup (map-select or beyond)
		},
		{
			Name: "select-map", Intent: "select map", Key: "a",
			NavKey: "Right", NavKeyBack: "Left", ParkUntilSelection: true,
			StepsFn:    func(s Selector) int { return s.MapPick().Steps },
			TargetName: func(s Selector) string { return s.MapPick().Name },
			CursorFn: func(o Observation) (int, int, bool) { return o.MapCursor, o.MapCursorCount, o.MapCursorValid },
			On:       hosting,
			Done:     inLobby, // only truly confirmable once the lobby is readable
			Blind:    true,
		},
		{
			Name: "select-gametype", Intent: "select gametype", Key: "a",
			NavKey: "Right", NavKeyBack: "Left",
			StepsFn:    func(s Selector) int { return s.GametypePick().Steps },
			TargetName: func(s Selector) string { return s.GametypePick().Name },
			CursorFn: func(o Observation) (int, int, bool) {
				return o.GametypeCursor, o.GametypeCursorCount, o.GametypeCursorValid
			},
			PrefixFn: gametypeCustomPrefix,
			On:       hosting,
			Done:     inLobby,
			Blind:    true,
		},
		{
			Name: "reach-lobby", Intent: "reach lobby", Key: "",
			On:   inLobby,
			Done: inLobby, // sentinel: confirms we're settled in the lobby
		},
	}, timing)
	seq.sel = sel
	return seq
}

// Cursor / Current expose progress for events + tests.
func (s *Sequence) Cursor() int { return s.cursor }
func (s *Sequence) Done() bool  { return s.cursor >= len(s.steps) }
func (s *Sequence) Current() (Transition, bool) {
	if s.Done() {
		return Transition{}, false
	}
	return s.steps[s.cursor], true
}

// Reset rewinds to the first step (used by the auto-host loop before re-hosting).
func (s *Sequence) Reset(now time.Time) {
	s.cursor = 0
	s.enteredAt = now
	s.pressed = false
	s.navDone = 0
	s.lastNavCursor = 0
	s.navFocus = 0
	s.navPresses = 0
	s.lastKey = ""
	s.navStuckItem = MenuItemUnknown
	s.navStuckCount = 0
	s.lastPress = time.Time{}
	s.sysLinkEntered = false
	s.sysLinkConnectAt = time.Time{}
}

func (s *Sequence) advance(now time.Time) {
	s.cursor++
	s.enteredAt = now
	s.pressed = false
	s.navDone = 0
	s.lastNavCursor = 0
	s.navFocus = 0
	s.navPresses = 0
	s.lastKey = ""
	s.navStuckItem = MenuItemUnknown
	s.navStuckCount = 0
	s.lastPress = time.Time{}
}

// Step decides the next Action for the current observation + clock:
//  1. catch-up: advance past any steps whose Done already holds (e.g. we landed
//     in the lobby, so every card step is retroactively satisfied);
//  2. Done when the sequence is complete;
//  3. a Blind card step (map/gametype — indistinguishable in memory) presses
//     once, then advances on a BlindAdvanceAfter timer — it CAN'T confirm the
//     sub-screen, so it never waits on On/Done between the cards;
//  4. a non-Blind step presses only while On holds (debounced) and advances on
//     Done; if On is false it Blocks rather than press blindly;
//  5. a key-less sentinel (reach-lobby) confirms the readable bracket.
func (s *Sequence) Step(obs Observation, now time.Time) Action {
	if !obs.Fresh {
		return wait("no fresh read")
	}
	// (1) catch up: advance past steps whose Done already holds (a box handed to us
	// mid-flow, or presses landing faster than timed).
	//
	// EXCEPT a selection card step (NavKey drive): its pick is applied by stepCard's
	// closed-loop drive-then-A, and it advances ITSELF once the card is committed —
	// never by a Done that merely reflects the lobby being readable. The host creates
	// the lobby with Y BEFORE picking a map, so the pregame sentinel makes inLobby
	// hold the whole time you're on the SELECT MAP / SELECT GAMETYPE screens; catch-
	// up-skipping there dropped the pick entirely ("no effect", diagnosed live
	// 2026-08-08). Stopping at the first card step lets stepCard drive it; a box that
	// is genuinely PAST selection holds on the "awaiting live cursor" wait (count 0
	// once the carousel deactivates) rather than silently applying a default.
	for !s.Done() {
		t := s.steps[s.cursor]
		if t.NavKey != "" {
			break
		}
		if t.Done != nil && t.Done(obs) {
			s.advance(now)
			continue
		}
		break
	}
	if s.Done() {
		return done("sequence complete")
	}

	t := s.steps[s.cursor]

	// (M) state-aware planner nav step (front-end → System Link, from anywhere).
	// Owns the whole front-end nav segment; routes by reading Observation.MenuItem.
	if t.Planner {
		return s.stepPlanNav(t, obs, now)
	}

	// (5) key-less sentinel: confirms we're settled on the bracket screen.
	if t.Key == "" {
		if t.On != nil && t.On(obs) {
			return done("reached " + t.Name)
		}
		return wait("awaiting " + t.Name)
	}

	// (P) PARK: the front-loaded map-select hold. The lobby is already created
	// (Y pressed, hosting advertised), so wait here on the readable map-select
	// checkpoint until the player has chosen — never a blind default.
	if t.ParkUntilSelection && (s.sel == nil || !s.sel.HasSelection()) {
		if t.On != nil && !t.On(obs) {
			return blocked(fmt.Sprintf("park step %q: hosting segment not observed (on %s)", t.Name, screenName(obs)))
		}
		return wait("parked at map-select — awaiting player map/gametype pick")
	}

	// (N) card step: CURSOR-RELATIVE + CLOSED-LOOP carousel navigation. Read the
	// live highlighted index, move toward the target one confirmed step at a time,
	// and press Key (A) only once the re-read cursor == target — never blind, never
	// an assumed default.
	if t.NavKey != "" {
		return s.stepCard(t, obs, now)
	}

	// (3) blind card step (no nav): gate entry on On, press once and advance on a
	// timer. Retained for compatibility; the default sequence uses NavKey steps.
	if t.Blind {
		if t.On != nil && !t.On(obs) {
			return blocked(fmt.Sprintf("blind step %q: hosting segment not observed (on %s)", t.Name, screenName(obs)))
		}
		if !s.pressed {
			s.pressed = true
			s.lastPress = now
			return tap(t.Key, t.Intent, "hosting card, pressing "+t.Key)
		}
		if now.Sub(s.lastPress) >= s.timing.BlindAdvanceAfter {
			s.advance(now)
			return s.Step(obs, now) // evaluate the next step this tick
		}
		return wait("blind " + t.Name + ", awaiting timed advance")
	}

	// (4) non-blind step: press only while On holds, debounced; else Block.
	if t.On != nil && t.On(obs) {
		if !s.pressed || now.Sub(s.lastPress) >= s.timing.RepressAfter {
			s.pressed = true
			s.lastPress = now
			return tap(t.Key, t.Intent, "on "+screenName(obs)+", pressing "+t.Key)
		}
		return wait("pressed " + t.Key + ", awaiting confirm")
	}
	return blocked(fmt.Sprintf("expected screen for %q not observed (on %s)", t.Name, screenName(obs)))
}

// navMaxPresses bounds the whole state-aware nav (route + recovery + dropped-
// press re-emits) before it blocks — generous for a route from an arbitrary
// screen (Back-normalise + main-menu + submenu + profile), but finite so a box
// that never navigates (no active network → Halo refuses System Link; or presses
// not reaching the guest) surfaces instead of looping forever.
const navMaxPresses = 48

// navStuckThreshold is how many consecutive emitted nav presses may leave the
// highlighted item UNCHANGED before the planner forces a Back to escape. It
// catches an off-route screen the classifier reads as a routable enum (so the
// planned key keeps firing without progress). It is well above the ~4–6 A-presses
// System Link legitimately takes to flip game_connection (Done short-circuits the
// nav before then), so a working confirm is never mistaken for stuck.
const navStuckThreshold = 8

// sysLinkConnectTimeout is how long the planner HOLDS after the single A on the
// SYSTEM LINK item while the connection establishes, without re-pressing. Selecting
// System Link Play flips game_connection 0→1 over several seconds (RUNTIME-OBSERVED
// ~6s on a networked box), during which the highlight does NOT change — so the
// normal "highlight changed = landed, else re-press" confirm would hammer A every
// RepressAfter and abort/restart the connect (the definitive "stuck on
// multiplayer_type_conn_item" bug). We wait for the screen/conn to actually move
// instead, and only re-press once this generous window elapses (true dropped press).
const sysLinkConnectTimeout = 20 * time.Second

// planNavKey chooses the next press purely from WHICH item is highlighted — the
// heart of the state-aware navigator. It never counts keys.
//
// The returned strings are logical KEY LABELS fed to the shared input path
// (internal/vncinput). They MUST be members of vncinput.KEYSYM — the d-pad is
// the CAPITALISED X11 keysym name ("Down", not "down"); the face buttons are
// lowercase ("a"/"b"/"y"). The rig's own /input RFB client is case-lenient, so a
// "down" was accepted there but REJECTED by cart's vncinput ("unknown key
// \"down\"") — every press dropped and the box never moved. Card nav already
// used "Right"/"Left"; this now matches. TestNavKeysAreVncinputLabels guards it.
func planNavKey(item MenuItem) string {
	switch item {
	case MenuItemMultiplayer: // main menu, on MULTIPLAYER → enter submenu
		return "a"
	case MenuItemSystemLink: // submenu, on SYSTEM LINK → enter (conn→1)
		return "a"
	case MenuItemProfile: // a profile screen — OFF the host-creation route.
		// Live on H1 Perf, choosing System Link reaches game_connection==1 with NO
		// profile-confirm step, and the only way you land on a profile widget is a
		// side path (Settings › Player Setup, Single Player › pick profile). Pressing
		// A there dives DEEPER and never progresses (an infinite-A trap observed
		// 2026-08-07). So Back-normalise out of it toward the main menu and re-plan.
		return "b"
	case MenuItemMainOther: // main menu, wrong item → step toward MULTIPLAYER
		return "Down"
	case MenuItemSubmenuOther: // submenu, wrong item → step toward SYSTEM LINK
		return "Down"
	default: // MenuItemUnknown / off-route (Settings, Single Player, unreadable)
		return "b" // Back-normalise toward the main menu, then re-plan
	}
}

// stepPlanNav is the STATE-AWARE front-end navigator. Every tick it reads which
// menu item is highlighted (Observation.MenuItem) and presses toward System Link
// by IDENTITY — Down toward MULTIPLAYER / SYSTEM LINK, A to enter / confirm, B to
// back out of an off-route screen — never a wrap-prone key count. Each press is
// confirmed by the menu-focus pointer (Observation.MenuFocus) changing; a dropped
// press is re-emitted until it lands. It completes the instant Done holds
// (game_connection≥1), blocks if a select mis-fired into a map/campaign
// (MenuActive→0), and blocks once navMaxPresses is spent (box not navigating).
func (s *Sequence) stepPlanNav(t Transition, obs Observation, now time.Time) Action {
	if t.Done != nil && t.Done(obs) { // reached System Link / hosting / lobby
		s.advance(now)
		return s.Step(obs, now)
	}
	// A select that mis-fired into a loaded map/campaign leaves the front-end:
	// MenuActive==0 while game_connection is still menu. Surface it, don't press on.
	if !obs.MenuActive && obs.Connection == ConnMenu {
		return blocked(fmt.Sprintf("nav step %q: left the front-end into a map/campaign (a select mis-fired) — MenuActive=0", t.Name))
	}
	if s.navPresses >= navMaxPresses {
		return blocked(fmt.Sprintf("nav step %q: %d presses without reaching system-link (highlighted=%s, focus 0x%08X) — box not navigating? active network? (Halo blocks System Link with no link)", t.Name, s.navPresses, obs.MenuItem, obs.MenuFocus))
	}

	key := planNavKey(obs.MenuItem)

	// (a) awaiting confirmation of the key we already emitted.
	if s.pressed {
		// SYSTEM LINK CONNECT window: after the single A on multiplayer_type_conn_item
		// the link takes ~6s to establish, and the highlight stays put the whole time.
		// So DON'T use focus-change here and DON'T re-press (a re-press aborts the
		// connect — the "28 A-presses, highlight never changes" bug). Land only when
		// the screen/conn actually moves; re-press once only if the generous window
		// elapses (the single A genuinely dropped). Done (reachedSystemLink) at the top
		// short-circuits this the instant conn→1.
		if !s.sysLinkConnectAt.IsZero() {
			// Advance ONLY when the reliable high-GVA HIGHLIGHT leaves the conn item
			// (onto a Select Profile widget, the server_list browser, or any other
			// screen). Do NOT consult game_connection — it flickers/reads stale here
			// and was falsely firing "connected — advancing" while the highlight never
			// moved, which then re-pressed A in a loop.
			if obs.MenuItem != MenuItemSystemLink {
				s.sysLinkConnectAt = time.Time{}
				s.pressed = false
				s.navDone++
				return wait(fmt.Sprintf("nav %s: System Link highlight moved — advancing (now %s)", t.Name, obs.MenuItem))
			}
			if now.Sub(s.sysLinkConnectAt) >= sysLinkConnectTimeout {
				s.sysLinkConnectAt = time.Time{}
				s.pressed = false
				return s.stepPlanNav(t, obs, now) // window elapsed → the A dropped, re-press once
			}
			return wait(fmt.Sprintf("nav %s: System Link connecting — holding, no re-press (%.0fs)", t.Name, now.Sub(s.sysLinkConnectAt).Seconds()))
		}
		if obs.MenuFocus != s.navFocus { // the highlight / screen changed → landed
			s.navDone++
			s.pressed = false
			return wait(fmt.Sprintf("nav %s: %q landed (now %s)", t.Name, s.lastKey, obs.MenuItem))
		}
		if now.Sub(s.lastPress) >= s.timing.RepressAfter { // dropped → re-emit
			s.pressed = false
			return s.stepPlanNav(t, obs, now)
		}
		return wait(fmt.Sprintf("nav %s: awaiting %q to land (on %s)", t.Name, s.lastKey, obs.MenuItem))
	}

	// (b) emit the planned key, paced by NavKeyInterval.
	if !s.lastPress.IsZero() && now.Sub(s.lastPress) < s.timing.NavKeyInterval {
		return wait(fmt.Sprintf("nav %s: pacing before %q", t.Name, key))
	}
	// stuck-detection: if the highlighted item hasn't changed for navStuckThreshold
	// consecutive emitted presses, the planned key isn't progressing on this screen
	// → force Back to escape toward the main menu, then re-plan from wherever we land.
	if obs.MenuItem == s.navStuckItem {
		s.navStuckCount++
	} else {
		s.navStuckItem = obs.MenuItem
		s.navStuckCount = 0
	}
	if s.navStuckCount >= navStuckThreshold {
		key = "b"
		s.navStuckCount = 0 // give the back-out a chance to change the screen
	}
	// SELECT PROFILE — the System Link entry flow (RE'd from Stewart's networked
	// box, 2026-08). After A on the SYSTEM LINK item (multiplayer_type_conn_item)
	// we pass through Select Profile at game_connection==0: join → select profile →
	// confirm, all A presses, until conn→1 lands us on the System Link games browser
	// (dela …\connected\server_list\…). Select Profile exposes no forward-routing
	// item (MenuItem=Unknown, or the profile widgets → MenuItemProfile), so the
	// planner would Back out of it. Instead ADVANCE with A — repeated until conn→1
	// (the profile sub-states don't need enumerating: each is one A, the screen
	// advances, and conn→1 is the terminator). Gated on sysLinkEntered so a
	// SIDE-PATH profile (Settings › Player Setup) still Backs out (no infinite-A).
	// The CREATE press itself is NOT here: it's create-game (Key "y"), which fires
	// once conn==1 (Classify → ScreenSystemLink) — so Y is keyed strictly to the
	// server_list browser and A is never pressed there (reachedSystemLink advances
	// the nav step the instant conn==1, before this planner can press).
	// Gate purely on the reliable high-GVA MenuItem, NOT game_connection — a stale
	// conn read must not misroute here. The System Link games browser is its own
	// MenuItemSystemLinkGames (reachedSystemLink advances the step to create-game's Y
	// before this runs), so it never matches Unknown/Profile → we never A (JOIN) it.
	if s.sysLinkEntered &&
		(obs.MenuItem == MenuItemUnknown || obs.MenuItem == MenuItemProfile) {
		key = "a"
	}
	// Remember we entered System Link, so the Select-Profile advance above kicks in,
	// and start the connect-hold window so we don't hammer A through the ~6s link-up.
	if key == "a" && obs.MenuItem == MenuItemSystemLink {
		s.sysLinkEntered = true
		s.sysLinkConnectAt = now
	}
	s.pressed = true
	s.navFocus = obs.MenuFocus
	s.lastKey = key
	s.lastPress = now
	s.navPresses++
	return tap(key, t.Intent, fmt.Sprintf("nav %s: %q — highlighted=%s (press %d)", t.Name, key, obs.MenuItem, s.navPresses))
}

// stepCard drives one card-select step (SELECT MAP / SELECT GAMETYPE) cursor-
// relative and closed-loop:
//
//  1. gate on On (must still be in the hosting segment);
//  2. read the LIVE cursor via CursorFn; if it isn't readable, HOLD (never press
//     blind) — the create-game screens expose the widget, so a miss is transient;
//  3. compute the shorter way around the (wrapping) carousel to the target index
//     (StepsFn) and press NavKey/NavKeyBack, but only once the previous move has
//     visibly landed (cursor changed) or RepressAfter elapsed — so a slow/dropped
//     press self-corrects and we never overshoot;
//  4. once the re-read cursor == target, press Key (A) and advance on the timer.
//
// Because the cursor is re-read every tick, the loop closes on confirmed state:
// A fires only when the highlighted card IS the pick, from any start, incl. wrap.
func (s *Sequence) stepCard(t Transition, obs Observation, now time.Time) Action {
	if t.On != nil && !t.On(obs) {
		return blocked(fmt.Sprintf("card step %q: hosting segment not observed (on %s)", t.Name, screenName(obs)))
	}

	cursor, count, ok := 0, 0, false
	if t.CursorFn != nil {
		cursor, count, ok = t.CursorFn(obs)
	}
	if !ok || count <= 0 {
		// No live cursor. If the finalized selection is already resident (Map AND
		// Gametype readable), we're PAST this card — settled in the lobby (or handed a
		// box mid-flow) — so advance rather than hold. This is what lets the catch-up
		// complete for an already-settled box now that the catch-up loop no longer
		// skips card steps by inLobby (which held on the card screens themselves).
		if obs.Map != "" && obs.Gametype != "" {
			s.advance(now)
			return s.Step(obs, now)
		}
		// Otherwise HOLD rather than navigate to a possibly-wrong card (replaces the
		// old blind default). On a card screen the carousel goes live within a tick;
		// during the create→card transition the pick isn't applied yet, so a hold —
		// not a skip — is correct (this is what stopped the "no effect" skip).
		return wait(fmt.Sprintf("card %s: awaiting live cursor read", t.Name))
	}

	target := 0
	name := ""
	if s.sel != nil {
		if t.StepsFn != nil {
			target = t.StepsFn(s.sel)
		}
		if t.TargetName != nil {
			name = t.TargetName(s.sel)
		}
	}
	// The pick's index is in the ENUMERATION space; shift it into the live WIDGET
	// space by the custom-variant prefix (0 for maps; the prepended-custom count
	// for gametypes) so the target lands on the right card.
	if t.PrefixFn != nil {
		target += t.PrefixFn(obs)
	}
	// Clamp the target into the live list range (defensive against a stale index).
	target = ((target % count) + count) % count

	right, remaining := carouselNav(cursor, target, count)
	if remaining > 0 {
		// Move toward the target. Press only when the last move has landed (cursor
		// changed since our last press) or the repress timer has elapsed.
		moved := s.navDone == 0 || cursor != s.lastNavCursor
		if moved || now.Sub(s.lastPress) >= s.timing.RepressAfter {
			s.navDone++
			s.lastNavCursor = cursor
			s.lastPress = now
			key := t.NavKey
			if !right && t.NavKeyBack != "" {
				key = t.NavKeyBack
			}
			return tap(key, t.Intent, fmt.Sprintf("nav %s → %q: cursor %d→%d (%d left)", t.Name, name, cursor, target, remaining))
		}
		return wait(fmt.Sprintf("card %s: nav pressed, awaiting cursor move (%d→%d)", t.Name, cursor, target))
	}

	// remaining == 0: the re-read cursor is ON the target — commit with A.
	if !s.pressed {
		s.pressed = true
		s.lastPress = now
		return tap(t.Key, t.Intent, fmt.Sprintf("cursor on target %d (%q), committing %s", target, name, t.Key))
	}
	if now.Sub(s.lastPress) >= s.timing.BlindAdvanceAfter {
		s.advance(now)
		return s.Step(obs, now) // evaluate the next step this tick
	}
	return wait("card " + t.Name + ": " + t.Key + " pressed, awaiting advance")
}

// carouselNav computes the D-pad move for a WRAPPING carousel from cursor to
// target over count items. It returns right=true to press forward (NavKey) or
// false to press backward (NavKeyBack), whichever way around the ring is shorter,
// and remaining = presses left (0 when already on target). The caller re-reads the
// live cursor each tick and re-invokes this, so a dropped/slow press just recomputes
// next tick — the loop is self-correcting and closes only when remaining hits 0.
//
// (target − cursor) mod count is the forward distance; count − that is the reverse.
// gametypeCustomPrefix is the SELECT GAMETYPE PrefixFn: the number of user-saved
// custom variants the carousel prepends ahead of the built-ins = live widget count
// − enumerated (built-in) count. Zero when the enumeration isn't available yet or
// the disc has no custom variants (stock), so a stock pick maps 1:1. Never
// negative.
func gametypeCustomPrefix(o Observation) int {
	if o.GametypeListLen <= 0 {
		return 0
	}
	if p := o.GametypeCursorCount - o.GametypeListLen; p > 0 {
		return p
	}
	return 0
}

func carouselNav(cursor, target, count int) (right bool, remaining int) {
	if count <= 0 {
		return true, 0
	}
	fwd := ((target-cursor)%count + count) % count
	if fwd == 0 {
		return true, 0
	}
	if bwd := count - fwd; bwd < fwd {
		return false, bwd
	}
	return true, fwd
}

// WalkBack presses the B face button to retreat one screen — and, if the native
// countdown is active, that same B cancels the countdown. It rewinds the cursor
// by one on a plain back-step so the machine re-presses forward from there.
func (s *Sequence) WalkBack(obs Observation, now time.Time) Action {
	if obs.CountdownActive {
		// The countdown owns the current screen; B stops it (no cursor change —
		// we stay in the lobby, just un-armed).
		return tap("b", "cancel countdown", "countdown active, pressing B to stop")
	}
	if s.cursor > 0 {
		s.cursor--
		s.enteredAt = now
		s.pressed = false
		s.navDone = 0
		s.lastNavCursor = 0
		s.lastPress = time.Time{}
	}
	return tap("b", "walk back", "pressing B to back out one screen")
}

func screenName(obs Observation) string { return Classify(obs).String() }

// vncinput labels used by the flow mirror the admin's XboxController.svelte /
// VNCKeyboard map exactly: A='a', B(face)='b', Y='y', Start='Return',
// Back='BackSpace'. In Halo menus the B FACE button is cancel/back, hence
// WalkBack emits "b".
