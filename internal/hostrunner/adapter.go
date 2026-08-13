package hostrunner

// ScraperReadout is the primitive bundle the scraper runner already has in its
// per-instance cache (read under cacheMu / via the loop's request-channel —
// never by touching a GameReader from a new goroutine). Keeping the mapping here
// in terms of primitives means hostrunner needs no scraper import and the whole
// adapter is unit-testable; the manager just fills these fields from its cache:
//
//	GameState       ← cache.GameState string ("menu"/"pregame"/"in_game"/"postgame")
//	MenuActive      ← ReadGameState StateInputs["main_menu"] != 0
//	GameConnection  ← game_connection global (0x2E3684): 0/1/2/3
//	Map, Gametype   ← cache.GameData.Map / .Gametype
//	MachineCount    ← len(cache.GameData.Machines)  (or TickNetworkGameData.MachineCount)
//	TeamCount       ← distinct teams in cache.GameData.TeamScores / Players
//	CountdownActive ← TickNetworkServer.CountdownActive != 0
//	CountdownPaused ← TickNetworkServer.CountdownPaused != 0
//
// Because game_connection / main_menu live at low GVAs, the manager must have
// re-run RefreshLowHVA across the XBE swap (menu↔game) before these reads are
// trustworthy — the same discipline the scraper already applies each Idle poll.
type ScraperReadout struct {
	Fresh           bool
	Tick            uint32
	GameState       string
	MenuActive      bool
	GameConnection  int
	Map             string
	Gametype        string
	MachineCount    int
	TeamCount       int
	PlayerCount     int
	CountdownActive bool
	CountdownPaused bool
	CardText        string

	// MenuFocus ← menu_focus (AddrUiWidgetFocusPtr 0x2F9B38): the CE front-end
	// menu widget-focus pointer. Only its CHANGES matter — the nav phase uses a
	// change as a "the menu press landed" confirmation (see lobby.go stepNav).
	MenuFocus uint32

	// MenuItem ← menu_item: WHICH front-end menu item is highlighted, as a
	// haloce.MenuItem* enum surfaced as an int (0=unknown/off-route,
	// 2=main-menu Multiplayer, 4=submenu System Link, 5=SELECT PROFILE, …). The
	// state-aware nav routes on it. 0 when off-route / not at the front-end.
	MenuItem int

	// LIVE create-game carousel cursors (CE widget +0x4C/+0x54), filled only while
	// hosting the setup screens. *Valid is false when the widget isn't readable —
	// the runner then holds rather than navigating on a stale index. See the
	// LobbyCursor projection in scraper/manager buildHostReadout.
	MapCursor           int
	MapCursorCount      int
	MapCursorValid      bool
	GametypeCursor      int
	GametypeCursorCount int
	GametypeCursorValid bool
	GametypeListLen     int

	// SCREEN-RECORD classifier (menu-nav pacing pass 2026-08-10). UiScreen is the
	// CURRENT screen's widget_definition tag path, resolved from the fixed
	// screen-record pool (AddrUiCurrentScreenRec 0x2E4000 → rec+0x00 tag id →
	// tag table) — 2 fixed reads + a cached resolve, so it's cheap enough for the
	// ~100ms host tick and it distinguishes visually identical screens the heap
	// fingerprint can't. "" = unreadable (cold boot / mod without the tags) →
	// Classify falls back to the heap-derived signals. UiBackScreenRec is the
	// record of the screen B returns to; it reads 0 EXACTLY at the root main
	// menu (non-zero = the cold-prime's "submenu opened" confirm; with UiScreen
	// it derives the at_root_menu diagnostic). The raw CURRENT-record pointer is
	// deliberately not carried — record slots are reused, so nothing downstream
	// may ever key on the address (it stays visible in the raw StateInputs debug
	// dump as "ui_screen_rec").
	UiScreen        string
	UiBackScreenRec uint32

	// UI support reads (diagnostic surfaces: OskActive = the on-screen keyboard
	// is capturing input, UiMsClock is the "UI alive" heartbeat, UiFadeState is
	// the D5/49 root ↔ D4/48 sub-screen byte pair).
	UiOskActive bool
	UiMsClock   uint32
	UiFadeState uint32

	// System Link entry-flow slot fields (port 0) — the per-A effect confirms
	// for join → select → commit. Persistent across flow exits; only meaningful
	// under the 4way screen record (EntryFrameOf applies the gate).
	SlotClaimed       bool
	SlotProfileHandle uint32

	// GameOver is the DEBOUNCED provisional game-over flag (AddrGameOverFlag —
	// the Server build's only readable game-end signal; it also drives
	// GameState postgame reader-side). Surfaced for the diagnostics panel.
	GameOver bool

	// Raw diagnostic reads for the admin panel (not used by the runner logic):
	// Dela is the highlighted-widget DeLa path (navfp `dela=`); PregameSentinel is
	// game_globals+0x10 == 0xDEADBEEF (pregame active).
	Dela            string
	PregameSentinel bool
	// Seq / ReadAtUnixMs stamp WHEN this readout was produced. They exist because
	// Tick (the GAME tick) is legitimately 0 at the front-end menus, so it can NEVER
	// answer "is the panel live?" — a frozen panel and a healthy menu both read 0.
	// Seq advances on every scraper tick, so it is the unambiguous liveness signal.
	Seq          uint64
	ReadAtUnixMs int64

	MainMenuRaw int // raw main_menu global (0/1) — cold-boot candidate
	// UI-widget-heap census (cold-vs-woken candidates): live block count, how many
	// carry the highlight flag, and the max activation tick ("UI tick").
	UIWidgetBlocks int
	UIHighlighted  int
	UIMaxTick      uint32

	// NavCandidates are raw, unclassified low globals from the capture-and-diff
	// hunts, surfaced on the panel for an operator to eyeball. Diagnostic only.
	NavCandidates map[string]uint32
}

// Observation projects a ScraperReadout into the runner's Observation. Unknown
// GameState strings map to PhaseMenu (the safe "not in a game" default) and
// GameConnection values outside 0..3 clamp to ConnMenu.
func (s ScraperReadout) Observation() Observation {
	phase := PhaseMenu
	switch Phase(s.GameState) {
	case PhasePreGame, PhaseInGame, PhasePostGame, PhaseMenu:
		phase = Phase(s.GameState)
	}
	conn := ConnMenu
	switch Connection(s.GameConnection) {
	case ConnSystemLink, ConnHosting, ConnFilm, ConnMenu:
		conn = Connection(s.GameConnection)
	}
	return Observation{
		Fresh:               s.Fresh,
		Tick:                s.Tick,
		Phase:               phase,
		MenuActive:          s.MenuActive,
		Connection:          conn,
		Map:                 s.Map,
		Gametype:            s.Gametype,
		MachineCount:        s.MachineCount,
		TeamCount:           s.TeamCount,
		PlayerCount:         s.PlayerCount,
		CountdownActive:     s.CountdownActive,
		CountdownPaused:     s.CountdownPaused,
		CardText:            s.CardText,
		MenuFocus:           s.MenuFocus,
		MenuItem:            MenuItem(s.MenuItem),
		MapCursor:           s.MapCursor,
		MapCursorCount:      s.MapCursorCount,
		MapCursorValid:      s.MapCursorValid,
		GametypeCursor:      s.GametypeCursor,
		GametypeCursorCount: s.GametypeCursorCount,
		GametypeCursorValid: s.GametypeCursorValid,
		GametypeListLen:     s.GametypeListLen,
		UiScreen:            s.UiScreen,
		UiBackScreenRec:     s.UiBackScreenRec,
		UiOskActive:         s.UiOskActive,
		UiMsClock:           s.UiMsClock,
		UiFadeState:         s.UiFadeState,
		SlotClaimed:         s.SlotClaimed,
		SlotProfileHandle:   s.SlotProfileHandle,
		GameOverFlag:        s.GameOver,
		Dela:                s.Dela,
		PregameSentinel:     s.PregameSentinel,
		MainMenuRaw:         s.MainMenuRaw,
		UIWidgetBlocks:      s.UIWidgetBlocks,
		UIHighlighted:       s.UIHighlighted,
		UIMaxTick:           s.UIMaxTick,
	}
}
