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

	// Raw diagnostic reads surfaced to the admin panel (NOT consumed by Classify or
	// the runner logic): Dela is the highlighted-widget DeLa path (navfp `dela=`);
	// PregameSentinel is game_globals+0x10 == 0xDEADBEEF (pregame active).
	Dela            string
	PregameSentinel bool
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
	// The highlighted widget under \connected\pregame is a definitive "this IS the
	// lobby" signal that overrides the sticky server_list, so classify it as the lobby.
	if strings.Contains(obs.Dela, `\connected\pregame`) {
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
	if obs.MenuItem == MenuItemSystemLinkGames {
		return ScreenSystemLink
	}
	// Front-end menu: a recognised highlighted front-end item (main menu / MP
	// submenu / SELECT PROFILE) means we're on the front-end regardless of the
	// possibly-stale low main_menu global.
	if obs.MenuItem.onFrontEndMenu() {
		return ScreenMainMenu
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
