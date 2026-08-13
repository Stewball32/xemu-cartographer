// Package hostrunner is the state-aware auto-host runner for the player-hosting
// feature: it composes the scraper's READ side (live Halo: CE lobby/menu state)
// with the internal/vncinput WRITE side (RFB KeyEvents into the container's
// Xvnc — the admin's shared channel) into a gated host-lobby state machine.
//
// Design principles baked into the types here:
//
//   - Never blind. Every menu transition is gated on readable state — the
//     scraper already reads game_connection (0 menu / 1 system-link / 2 hosting),
//     main_menu_active, the connected-machine (box) count, team count, the
//     selected map/gametype, and the native countdown flags. The machine
//     confirms "am I on the screen I think I am" before each press. The only
//     blind segment — the map/gametype card screens, which expose no
//     distinguishing global — is TIMED but bracketed by readable checkpoints
//     (entered hosting → landed in a lobby with map/gametype/machines readable).
//
//   - Native start conditions are READ, not controlled. The countdown needs
//     2+ boxes, 2+ teams and a finalized map/gametype — all native Halo. The
//     runner only times the arm (A on gametype) and start (A again) presses and
//     exposes the counts; it never fakes them (see start.go).
//
//   - Pure logic, fully unit-tested. All decision logic here is a pure function
//     of an Observation (+ config + clock), so it is testable with fakes and no
//     live container. I/O (state read, key emit, event broadcast) lives behind
//     the interfaces in runner.go.
//
// The package is decoupled from the scraper's concrete types: an integration
// adapter maps the scraper cache/GameData → Observation (see
// ObservationFromScraper's doc in runner.go). This keeps the machine self-
// contained and the scraper reader free to evolve.
package hostrunner

import "strings"

// Phase mirrors the scraper's GameState string values so the integration adapter
// is a trivial cast. It is the coarse engine lifecycle the auto-host loop reacts
// to.
type Phase string

const (
	PhaseMenu     Phase = "menu"
	PhasePreGame  Phase = "pregame"
	PhaseInGame   Phase = "in_game"
	PhasePostGame Phase = "postgame"
)

// Connection is the Halo CE game_connection global (AddrGameConnection 0x2E3684).
type Connection int

const (
	ConnMenu       Connection = 0 // front-end menu / single player
	ConnSystemLink Connection = 1 // system-link browser / join
	ConnHosting    Connection = 2 // hosting a game (setup + lobby)
	ConnFilm       Connection = 3 // film playback
)

// Observation is the readable snapshot the runner decides on each tick. It is
// built from the scraper runner's cache (never by touching a GameReader from a
// new goroutine — see runner.go). All fields are what the CE scraper already
// reads; CardText is an optional future menu-card-text hint (empty when the
// reader can't distinguish the map/gametype sub-screens).
type Observation struct {
	// Fresh is false when there is no live read this tick (init, XBE swap in
	// progress, read error). The runner never acts on a stale observation.
	Fresh bool
	Tick  uint32

	Phase           Phase
	MenuActive      bool
	Connection      Connection
	Map             string
	Gametype        string
	MachineCount    int // connected consoles/boxes (native start needs >= 2)
	TeamCount       int // distinct teams present (native start needs >= 2)
	PlayerCount     int
	CountdownActive bool
	CountdownPaused bool

	// CardText is an optional hint for the blind map/gametype card screens. ""
	// when unavailable; when a reader provides it, Classify prefers it.
	CardText string

	// MenuFocus is the CE front-end menu widget-focus pointer (menu_focus,
	// AddrUiWidgetFocusPtr 0x2F9B38). Its value is a relinking heap pointer — NOT
	// a stable index — but it reliably changes when the highlighted menu item
	// moves and stays put when a press dropped. The nav phase uses that change as
	// a per-press "the move landed" confirmation (stepNav), so a dropped d-pad
	// press can't leave the runner firing A on the wrong item.
	MenuFocus uint32

	// MenuItem is WHICH front-end menu item is currently highlighted, read from
	// the CE UI widget heap (the +0x60 highlight flag → the item's DeLa tag path,
	// classified). It lets the nav route by item IDENTITY (navigate until
	// MULTIPLAYER is highlighted, then A) instead of a wrap-prone key count, and
	// self-correct from any starting screen. The int values MIRROR
	// haloce.MenuItem* (kept in sync deliberately; the adapter passes the raw int
	// to avoid a scraper→hostrunner import cycle).
	MenuItem MenuItem

	// LIVE create-game carousel cursors (the highlighted-card index + list length
	// read directly from the CE menu widget system). *Valid is true only when the
	// corresponding SELECT MAP / SELECT GAMETYPE list widget is active this read.
	// These make card navigation cursor-relative + closed-loop: the runner presses
	// toward *Cursor==target and confirms the re-read landed before committing (A),
	// so no fixed default and any non-deterministic start is handled. Empty/false
	// off the create-game screens (the card steps then hold, never press blind).
	MapCursor           int
	MapCursorCount      int
	MapCursorValid      bool
	GametypeCursor      int
	GametypeCursorCount int
	GametypeCursorValid bool

	// GametypeListLen is the number of gametypes in the player-picker enumeration
	// (the ustr built-in set). The SELECT GAMETYPE widget carousel PREPENDS any
	// user-saved custom variants ahead of the built-ins, so its live count exceeds
	// this; the difference (GametypeCursorCount − GametypeListLen) is the custom
	// prefix the runner adds to a pick's ustr index to hit the right widget card.
	// Maps have no such prefix (widget index == ustr index), so no map equivalent.
	GametypeListLen int

	// UiScreen is the CURRENT screen's widget_definition tag path from the CE
	// screen-record pool (AddrUiCurrentScreenRec 0x2E4000 → rec+0x00 tag id →
	// tag table; menu-nav pacing pass 2026-08-10). Two fixed low reads + a cached
	// resolve — the cheap, authoritative "which screen" signal Classify prefers
	// over the UI-heap DeLa fingerprint, and the only signal that distinguishes
	// visually identical screens (campaign ENTER NAME vs profiles ENTER NAME).
	// "" = unreadable → Classify falls back to the heap-derived signals.
	UiScreen string
	// UiBackScreenRec is the record of the screen B returns to
	// (AddrUiBackScreenRec 0x2E4010); it reads 0 EXACTLY at the root main menu.
	// Going non-zero is the cold-menu prime's record-gated "a submenu opened"
	// confirm after its tree-building A; with UiScreen it also derives the
	// AtRootMainMenu diagnostic.
	UiBackScreenRec uint32
	// UiOskActive is 1 while the on-screen keyboard is capturing input (presses
	// land in a text buffer, not menu nav). Diagnostic surface for now.
	UiOskActive bool
	// UiMsClock is the free-running ms-scale UI clock — the correct "UI alive"
	// heartbeat. Diagnostic surface for now.
	UiMsClock uint32
	// UiFadeState is the transition byte pair (D5/49 root ↔ D4/48 sub-screen,
	// read as LE u16). Diagnostic surface.
	UiFadeState uint32

	// System Link ENTRY-FLOW slot fields (port 0; mapper 2026-08-11 §1). The
	// per-A effect confirms for the join → select → commit ladder: SlotClaimed
	// flips 0→1 on the claim A (~173ms), SlotProfileHandle goes -1 → handle on
	// the select A (~163ms), and the commit A flips the record + back-rec +
	// game_connection in the same frame. PERSISTENT across flow exits — only
	// meaningful while InSysLinkEntryFlow() (the 4way record); entryFrameOf
	// applies that gate.
	SlotClaimed       bool
	SlotProfileHandle uint32

	// GameOverFlag mirrors the debounced provisional game-over read (the same
	// signal that flips Phase to postgame reader-side) — a diagnostic surface;
	// the runner routes on Phase.
	GameOverFlag bool

	// Raw diagnostic reads surfaced to the admin panel (NOT consumed by Classify or
	// the runner logic): Dela is the highlighted-widget DeLa path (navfp `dela=`);
	// PregameSentinel is game_globals+0x10 == 0xDEADBEEF (pregame active).
	Dela            string
	PregameSentinel bool
	MainMenuRaw     int
	UIWidgetBlocks  int
	UIHighlighted   int
	UIMaxTick       uint32
}

// uiPathMainMenuRoot is the root main menu's screen-record tag path (resolved
// live on ce-h1perf, 2026-08-10). AtRootMainMenu requires the CURRENT screen to
// resolve to exactly this — never infer root from back-rec==0 alone, because a
// cold/unreadable record pool also reads 0.
const uiPathMainMenuRoot = `ui\shell\main_menu\main_menu`

// AtRootMainMenu reports a POSITIVE screen-record confirmation that the box sits
// on the root main menu: the current screen resolves to the main-menu tag AND
// the back-screen record is 0 (which is true exactly there). Surfaced on the
// admin diagnostics (`at_root_menu`) so a live walk can verify the record reads;
// the cold-menu prime itself runs its fixed check-B → check-Down → check-A
// ladder regardless (see lobby.go), so nothing routes on this.
func (obs Observation) AtRootMainMenu() bool {
	return obs.UiScreen == uiPathMainMenuRoot && obs.UiBackScreenRec == 0
}

// screenFromUiPath maps a screen-record tag path onto the host-flow Screen.
// Substring rules mirror the verified heap-signal markers; the hosting-flow
// paths were live-confirmed on beta 2026-08-11 (`connected_map_select_wrapper`
// is the System Link create-flow SELECT MAP screen, `server_list_screen` the
// games browser, `4way_start2join_screen` the entry flow). Classify still
// consults this AFTER the runtime-verified heap signals, as gap-fill rather
// than override:
//
//	…\connected\pregame…       → the settled pregame lobby
//	…\connected\server_list…   → the System Link games browser (CREATE screen)
//	…connected_map_select… / …mp_map_select… / …gametype_select… → hosting cards
//	ui\shell\main_menu…        → any front-end shell screen (root, submenus,
//	                             profile flows, ENTER NAME, 4way join, …)
func screenFromUiPath(path string) Screen {
	switch {
	case path == "":
		return ScreenUnknown
	case strings.Contains(path, `\connected\pregame`):
		return ScreenLobby
	case strings.Contains(path, `\connected\server_list`):
		return ScreenSystemLink
	case strings.Contains(path, `connected_map_select`),
		strings.Contains(path, `mp_map_select`),
		strings.Contains(path, `gametype_select`):
		return ScreenHosting
	case strings.HasPrefix(path, `ui\shell\main_menu`):
		return ScreenMainMenu
	}
	return ScreenUnknown
}

// InSysLinkEntryFlow reports the screen record resolving to the System Link
// ENTRY flow — the whole join → select → commit ladder runs under the ONE
// 4way_start2join record ("4way" = the four LOCAL controller quadrants, not
// network machines; the record does NOT flip per sub-screen). The heap
// highlight stays the STALE conn item throughout — no 4way widget carries the
// item kind bit — so the record is the only screen truth here, and the slot
// fields (SlotClaimed / SlotProfileHandle) are the per-A truth.
func (obs Observation) InSysLinkEntryFlow() bool {
	return strings.Contains(obs.UiScreen, `4way`)
}

// slotProfileNone is AddrUiSlotProfile's "no profile selected" sentinel.
const slotProfileNone = 0xFFFFFFFF

// EntryFrame is which step of the System Link entry ladder the port-0 slot is
// on, classified from the slot-field frame table (mapper §1 — cold-attachable
// mid-flow; verified on five independently-staged rigs):
//
//	initial:  4way record, slot not claimed        → next press: claim A
//	claimed:  4way, claimed, handle == none        → next press: select A
//	selected: 4way, claimed, handle != none        → next press: commit A
//
// EntryNone outside the 4way record — the slot fields DO NOT reset on flow
// exit, so they must never be read without this gate.
type EntryFrame int

const (
	EntryNone EntryFrame = iota
	EntryInitial
	EntryClaimed
	EntrySelected
)

func (f EntryFrame) String() string {
	switch f {
	case EntryInitial:
		return "initial"
	case EntryClaimed:
		return "claimed"
	case EntrySelected:
		return "selected"
	default:
		return "none"
	}
}

// EntryFrameOf classifies the observation onto the entry-ladder frame table.
func EntryFrameOf(obs Observation) EntryFrame {
	if !obs.InSysLinkEntryFlow() {
		return EntryNone
	}
	switch {
	case !obs.SlotClaimed:
		return EntryInitial
	case obs.SlotProfileHandle == slotProfileNone:
		return EntryClaimed
	default:
		return EntrySelected
	}
}

// MenuItem is which front-end menu item is highlighted, read from the CE UI
// widget heap. The values MIRROR internal/scraper/haloce's MenuItem* constants
// (the adapter passes the raw int across the package boundary — keep in sync).
type MenuItem int

const (
	MenuItemUnknown      MenuItem = 0 // no recognised front-end item highlighted / off-route
	MenuItemMainOther    MenuItem = 1 // main menu, a non-Multiplayer item
	MenuItemMultiplayer  MenuItem = 2 // main menu, MULTIPLAYER highlighted
	MenuItemSubmenuOther MenuItem = 3 // Multiplayer submenu, a non-System-Link item
	MenuItemSystemLink   MenuItem = 4 // Multiplayer submenu, SYSTEM LINK (conn) highlighted
	MenuItemProfile      MenuItem = 5 // SELECT PROFILE screen
	// MenuItemSystemLinkGames: the System Link games browser (…\connected\server_list),
	// read from the reliable high-GVA widget heap — the screen where the host presses
	// Y to CREATE. Recognising it by widget lets Classify return ScreenSystemLink
	// WITHOUT the stale-prone game_connection low global (which reads 0 here).
	MenuItemSystemLinkGames MenuItem = 6
)

func (m MenuItem) String() string {
	switch m {
	case MenuItemMainOther:
		return "main_menu_other"
	case MenuItemMultiplayer:
		return "main_menu_multiplayer"
	case MenuItemSubmenuOther:
		return "mp_submenu_other"
	case MenuItemSystemLink:
		return "mp_submenu_system_link"
	case MenuItemProfile:
		return "select_profile"
	case MenuItemSystemLinkGames:
		return "system_link_games"
	default:
		return "unknown"
	}
}

// onFrontEndMenu reports whether the box is on a recognised front-end menu screen
// the state-aware nav knows how to route from (main menu or Multiplayer submenu
// or SELECT PROFILE). MenuItemUnknown ⇒ off-route (Settings / a campaign screen /
// a screen we can't read) → the nav Back-normalises toward the main menu.
func (m MenuItem) onFrontEndMenu() bool { return m != MenuItemUnknown }

// Screen is the classified host-flow screen. The map/gametype select screens
// collapse into ScreenHosting because no memory global distinguishes them (the
// runner times those presses between readable brackets).
type Screen int

const (
	ScreenUnknown Screen = iota
	ScreenMainMenu
	ScreenSystemLink // system-link browser (game_connection == 1)
	ScreenHosting    // hosting setup: map/gametype card screens (blind)
	ScreenLobby      // pregame lobby: map/gametype/machines readable
	ScreenInGame
	ScreenPostGame
)

func (s Screen) String() string {
	switch s {
	case ScreenMainMenu:
		return "main_menu"
	case ScreenSystemLink:
		return "system_link"
	case ScreenHosting:
		return "hosting"
	case ScreenLobby:
		return "lobby"
	case ScreenInGame:
		return "in_game"
	case ScreenPostGame:
		return "post_game"
	default:
		return "unknown"
	}
}

// Classify maps an Observation onto a host-flow Screen using only readable
// state. Order matters: engine phase wins over menu heuristics (an in_game /
// post-game read is unambiguous), then connection + lobby-readable fields.
func Classify(obs Observation) Screen {
	if !obs.Fresh {
		return ScreenUnknown
	}
	switch obs.Phase {
	case PhaseInGame:
		return ScreenInGame
	case PhasePostGame:
		return ScreenPostGame
	}
	// PREGAME LOBBY override. The pregame lobby (SELECT TEAMS) lingers the System Link
	// server_list widgets, which would otherwise re-detect as the CREATE screen
	// (menu_item=SystemLinkGames) and re-press Y — RUNTIME-OBSERVED on a box that
	// BOOTED INTO a persisted pregame lobby (Stewart: dela=…\connected\pregame\…,
	// menu_item=6, serverlist=true, conn=2 → runner looped "create system-link game").
	// Signalled by the SCREEN RECORD or the dela under \connected\pregame — the
	// record is the primary since the item-kind highlight pick correctly reports
	// "no live item" on the pregame screen (its only highlighted block is the
	// xbox_graphic DECORATION — mapper 2026-08-11 §7), leaving dela empty there.
	// Safe ahead of the cursor checks: the create-flow card screens carry their own
	// records (connected_map_select_wrapper — live-verified), never the pregame one.
	if strings.Contains(obs.Dela, `\connected\pregame`) ||
		strings.Contains(obs.UiScreen, `\connected\pregame`) ||
		strings.Contains(obs.UiScreen, `connected_pregame`) {
		return ScreenLobby
	}
	// HIGH-GVA screen signals FIRST — read from the UI widget heap via the stable
	// physical 0x80000000-window, immune to the low-GVA cached-translation drift
	// that makes game_connection / main_menu read stale-zero and blank the runner
	// (the stall). These win over the low globals.
	//
	// Hosting: the SELECT MAP / SELECT GAMETYPE list widgets are live this read.
	// This also makes reachedSystemLink true the moment SELECT MAP loads, so the
	// nav completes without depending on a fresh game_connection==1 read.
	//
	// The SELECT MAP / SELECT GAMETYPE list widget being UP (cursor Count>0) means
	// we're actively on that card screen — NOT the settled team lobby, so return
	// ScreenHosting (never ScreenLobby) even though lobbyReadable() is true here. The
	// host creates the lobby with Y BEFORE picking a map (front-loaded design), so
	// the pregame sentinel is set the whole time you're on the card screens —
	// RUNTIME-OBSERVED 2026-08-08 on the live rig and CONFIRMED by Stewart's beta.log
	// (c0b80a3): on the Select Map screen game_globals+0x10 reads 0xDEADBEEF
	// (Phase==PreGame → lobbyReadable) while the map list is up, so the old
	// `lobbyReadable → ScreenLobby` branch returned ScreenLobby ON the card screen and
	// the runner declared "lobby ready — players start" the instant it landed on
	// Select Map, never driving the carousels → the pick had "no effect". Keying on
	// Count>0 (not Valid) keeps this true through a scroll animation, when the settled
	// index briefly reads out-of-range. The settled Select Teams lobby has NO list up
	// (Count 0) and still classifies as ScreenLobby via the fallback below.
	if obs.MapCursorCount > 0 || obs.GametypeCursorCount > 0 {
		return ScreenHosting
	}
	// System Link games browser, recognised by its high-GVA widget path
	// (…\connected\server_list). This is the CREATE screen — return ScreenSystemLink
	// off the RELIABLE widget, NOT game_connection, which reads stale-0 here in the
	// runner's fast loop (the same low-global drift as the original stall). Without
	// this, create-game's "Y at conn==1" never fired and the runner pressed A (JOIN).
	// NOT while the record shows the 4way ENTRY flow: the browser's list widgets
	// LINGER after a prior visit (presence-detection), so on a re-entry the flow
	// would otherwise classify as the CREATE screen and fire Y mid-join.
	if obs.MenuItem == MenuItemSystemLinkGames && !obs.InSysLinkEntryFlow() {
		return ScreenSystemLink
	}
	// Front-end menu: a recognised highlighted front-end item (main menu / MP
	// submenu / SELECT PROFILE) means we're on the front-end regardless of the
	// possibly-stale low main_menu global.
	if obs.MenuItem.onFrontEndMenu() {
		return ScreenMainMenu
	}
	// SCREEN-RECORD classifier (2026-08-10): the resolved current-screen tag
	// path. Placed AFTER the runtime-verified heap signals (cursor counts /
	// server-list presence / highlighted item) so it can never override them —
	// its job is the gaps they leave: a cold main menu whose widget tree isn't
	// built yet (record readable, dela blank — classifies as the front-end
	// WITHOUT waking the tree), the campaign/profile ENTER NAME screens, the
	// 4way join flow, and any shell screen with an unreadable highlight. This
	// is what lets the fast host tick classify from 2 fixed reads.
	if s := screenFromUiPath(obs.UiScreen); s != ScreenUnknown {
		return s
	}
	// LOW-GVA fallback. The system-link browser is NOT classified from
	// game_connection anymore — that low global flickers/reads stale in the runner's
	// fast loop and was poisoning the whole entry→create path (falsely "reaching"
	// system-link / creating on the wrong screen). System-link is now determined
	// SOLELY by the high-GVA server_list widget (MenuItemSystemLinkGames, above).
	// Only the post-create hosting fallback remains (and cursor-validity above is the
	// primary hosting signal); main_menu is belt-and-suspenders behind MenuItem.
	switch obs.Connection {
	case ConnMenu:
		if obs.MenuActive {
			return ScreenMainMenu
		}
		return ScreenUnknown
	case ConnHosting:
		if lobbyReadable(obs) {
			return ScreenLobby
		}
		return ScreenHosting
	}
	return ScreenUnknown
}

// lobbyReadable reports whether the pregame lobby's fields are resident — a
// finalized map/gametype, a connected-machine roster, or the engine sitting in
// pregame — which distinguishes ScreenLobby from the blind card screens.
func lobbyReadable(obs Observation) bool {
	return obs.Phase == PhasePreGame || obs.MachineCount > 0 || (obs.Map != "" && obs.Gametype != "")
}

// ReadyToStart reports whether Halo's NATIVE countdown preconditions are met:
// 2+ connected boxes and 2+ teams. This is READ, never controlled — the runner
// exposes it and only decides whether to press start when it holds.
func (obs Observation) ReadyToStart() bool {
	return obs.MachineCount >= 2 && obs.TeamCount >= 2
}
