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
	}
}
