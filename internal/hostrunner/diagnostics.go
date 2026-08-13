package hostrunner

import "time"

// CursorView is one carousel cursor read (CE widget +0x4C selected / +0x54 count)
// for the admin diagnostics panel. Valid is false when the list isn't the live
// foreground (count 0) or the index is mid-scroll out of range.
type CursorView struct {
	Index int  `json:"index"`
	Count int  `json:"count"`
	Valid bool `json:"valid"`
}

// Diagnostics is the live per-tick scraper-read snapshot for the admin diagnostics
// panel — the RAW reads behind the classified screen, so an operator can watch the
// box AND what the scraper sees side-by-side and report a fingerprint for any
// screen without grepping beta.log. Built from the last captured RunnerEvent
// (updated EVERY tick, not just on decision-change) plus the runner's live pick.
type Diagnostics struct {
	Instance string `json:"instance"`
	Present  bool   `json:"present"`
	Tick     uint32 `json:"tick"` // GAME tick — legitimately 0 at the front-end menus
	// ReadoutSeq advances on EVERY scraper tick and ReadoutAgeMs is how long ago the
	// scraper produced this readout. These — not Tick — are the liveness signals: a
	// healthy box sitting in the menus reads Tick 0 forever, so Tick alone can't
	// distinguish "live at a menu" from "panel frozen" (exactly the confusion that
	// masked the frozen-panel bug).
	ReadoutSeq      uint64 `json:"readout_seq"`
	ReadoutAgeMs    int64  `json:"readout_age_ms"`
	Screen          string `json:"screen"`
	Dela            string `json:"dela"`      // highlighted-widget DeLa path (navfp dela=)
	MenuItem        int    `json:"menu_item"` // resolved MenuItem enum
	MenuItemName    string `json:"menu_item_name"`
	GameConnection  int    `json:"game_connection"`  // conn: 0 menu, 1 system-link, 2 hosting, 3 film
	PregameSentinel bool   `json:"pregame_sentinel"` // game_globals+0x10 == 0xDEADBEEF
	MenuFocus       uint32 `json:"menu_focus"`       // 0x2F9B38 focused-widget ptr — 0 on a cold un-woken menu
	MainMenuRaw     int    `json:"main_menu_raw"`    // raw main_menu global
	UIWidgetBlocks  int    `json:"ui_widget_blocks"` // live widget blocks — the cold-vs-woken discriminator
	UIHighlighted   int    `json:"ui_highlighted"`   // blocks carrying the +0x60 highlight flag
	UIMaxTick       uint32 `json:"ui_max_tick"`      // max widget activation tick ("UI tick")
	TreeBuilt       bool   `json:"tree_built"`       // derived: the widget tree is up (not a cold menu)
	// Screen-record classifier + UI support reads (2026-08-10 pacing pass).
	UiScreen        string `json:"ui_screen"`          // resolved current-screen tag path ("" = no signal)
	UiBackScreenRec uint32 `json:"ui_back_screen_rec"` // back-screen record — 0 exactly at the root menu
	AtRootMenu      bool   `json:"at_root_menu"`       // derived: screen record positively confirms the root main menu
	UiOskActive     bool   `json:"ui_osk_active"`      // on-screen keyboard is capturing input
	UiMsClock       uint32 `json:"ui_ms_clock"`        // free-running UI ms clock — the "UI alive" heartbeat
	UiFadeState     uint32 `json:"ui_fade_state"`      // D5/49 root ↔ D4/48 sub-screen byte pair
	// Entry-flow slot fields + the classified ladder frame (2026-08-11): the
	// per-A truth of the System Link join → select → commit ladder.
	SlotClaimed       bool   `json:"slot_claimed"`
	SlotProfileHandle uint32 `json:"slot_profile_handle"`
	EntryFrame        string `json:"entry_frame"`
	GameOverFlag      bool   `json:"game_over_flag"` // debounced provisional 0x277B94 (postgame trigger)
	// NavCandidates: raw candidate offsets from the capture-and-diff hunts, for an
	// operator to watch live and tell us which one tracks. Diagnostic only.
	NavCandidates map[string]uint32 `json:"nav_candidates"`

	MapCursor      CursorView `json:"map_cursor"`
	GametypeCursor CursorView `json:"gametype_cursor"`

	// Map/Gametype are the READ (currently-loaded) values; SelectedMap/Gametype are
	// the player's picked intent off the runner's selector.
	Map              string `json:"map"`
	Gametype         string `json:"gametype"`
	SelectedMap      string `json:"selected_map"`
	SelectedGametype string `json:"selected_gametype"`

	// What the runner just DID + the live roster counts — so the panel shows the
	// decision alongside the reads that drove it.
	LastKind        string   `json:"last_kind"`
	LastIntent      string   `json:"last_intent"`
	LastKeys        []string `json:"last_keys"`
	LastReason      string   `json:"last_reason"`
	MachineCount    int      `json:"machine_count"`
	PlayerCount     int      `json:"player_count"`
	TeamCount       int      `json:"team_count"`
	CountdownActive bool     `json:"countdown_active"`
	Authority       string   `json:"authority"`
}

// ApplyReadout overwrites the LIVE-READ fields from the current tick's readout.
// The runner-decision fields (last action/reason, authority, selection) still come
// from the registry's captured event; everything an operator watches per-tick comes
// from here, so the panel stays live even when no host runner is attached (the frozen
// "tick 0" panel bug: the registry event only exists while a runner ticks).
func (d *Diagnostics) ApplyReadout(ro ScraperReadout) {
	obs := ro.Observation()
	d.Tick = ro.Tick
	d.ReadoutSeq = ro.Seq
	if ro.ReadAtUnixMs > 0 {
		d.ReadoutAgeMs = time.Now().UnixMilli() - ro.ReadAtUnixMs
	}
	d.Screen = Classify(obs).String()
	d.Dela = ro.Dela
	d.MenuItem = ro.MenuItem
	d.MenuItemName = MenuItem(ro.MenuItem).String()
	d.GameConnection = ro.GameConnection
	d.PregameSentinel = ro.PregameSentinel
	d.MenuFocus = ro.MenuFocus
	d.MainMenuRaw = ro.MainMenuRaw
	d.UIWidgetBlocks = ro.UIWidgetBlocks
	d.UIHighlighted = ro.UIHighlighted
	d.UIMaxTick = ro.UIMaxTick
	d.UiScreen = ro.UiScreen
	d.UiBackScreenRec = ro.UiBackScreenRec
	d.AtRootMenu = obs.AtRootMainMenu()
	d.UiOskActive = ro.UiOskActive
	d.UiMsClock = ro.UiMsClock
	d.UiFadeState = ro.UiFadeState
	d.SlotClaimed = ro.SlotClaimed
	d.SlotProfileHandle = ro.SlotProfileHandle
	d.EntryFrame = EntryFrameOf(obs).String()
	d.GameOverFlag = ro.GameOver
	d.NavCandidates = ro.NavCandidates
	d.TreeBuilt = ro.Dela != "" || ro.MenuFocus != 0 || ro.UIHighlighted > 0
	d.MapCursor = CursorView{Index: ro.MapCursor, Count: ro.MapCursorCount, Valid: ro.MapCursorValid}
	d.GametypeCursor = CursorView{Index: ro.GametypeCursor, Count: ro.GametypeCursorCount, Valid: ro.GametypeCursorValid}
	d.Map = ro.Map
	d.Gametype = ro.Gametype
	d.MachineCount = ro.MachineCount
	d.PlayerCount = ro.PlayerCount
	d.TeamCount = ro.TeamCount
	d.CountdownActive = ro.CountdownActive
}

// Diagnostics returns the live scraper-read snapshot for an instance, built from
// the last captured event (every tick) + the runner's current selection. Present
// is false with a zero snapshot when no runner is attached for the name.
func (reg *Registry) Diagnostics(instance string) Diagnostics {
	reg.mu.RLock()
	r, present := reg.runners[instance]
	ev, hasEv := reg.last[instance]
	reg.mu.RUnlock()

	d := Diagnostics{Instance: instance, Present: present}
	if present {
		mp, gt := r.Selection()
		d.SelectedMap = mp.Name
		d.SelectedGametype = gt.Name
	}
	if hasEv {
		d.Tick = ev.Tick
		d.Screen = ev.Screen
		d.Dela = ev.Dela
		d.MenuItem = ev.MenuItem
		d.MenuItemName = MenuItem(ev.MenuItem).String()
		d.GameConnection = ev.GameConnection
		d.PregameSentinel = ev.PregameSentinel
		d.MenuFocus = ev.MenuFocus
		d.MainMenuRaw = ev.MainMenuRaw
		d.UIWidgetBlocks = ev.UIWidgetBlocks
		d.UIHighlighted = ev.UIHighlighted
		d.UIMaxTick = ev.UIMaxTick
		d.UiScreen = ev.UiScreen
		d.UiBackScreenRec = ev.UiBackScreenRec
		d.AtRootMenu = ev.UiScreen == uiPathMainMenuRoot && ev.UiBackScreenRec == 0
		d.UiOskActive = ev.UiOskActive
		d.UiMsClock = ev.UiMsClock
		d.UiFadeState = ev.UiFadeState
		d.SlotClaimed = ev.SlotClaimed
		d.SlotProfileHandle = ev.SlotProfileHandle
		d.EntryFrame = ev.EntryFrame
		d.GameOverFlag = ev.GameOverFlag
		// "Widget tree built?" — the cold-boot go/no-go the prime keys on: a readable
		// highlight, a focused widget, or a populated heap all mean it's awake.
		d.TreeBuilt = ev.Dela != "" || ev.MenuFocus != 0 || ev.UIHighlighted > 0
		d.MapCursor = CursorView{Index: ev.MapCursor, Count: ev.MapCursorCount, Valid: ev.MapCursorValid}
		d.GametypeCursor = CursorView{Index: ev.GametypeCursor, Count: ev.GametypeCursorCount, Valid: ev.GametypeCursorValid}
		d.Map = ev.Map
		d.Gametype = ev.Gametype
		d.LastKind = ev.Kind
		d.LastIntent = ev.Intent
		d.LastKeys = ev.Keys
		d.LastReason = ev.Reason
		d.MachineCount = ev.MachineCount
		d.PlayerCount = ev.PlayerCount
		d.TeamCount = ev.TeamCount
		d.CountdownActive = ev.CountdownActive
		d.Authority = ev.Authority
	}
	return d
}
