package hostrunner

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
	Instance        string `json:"instance"`
	Present         bool   `json:"present"`
	Tick            uint32 `json:"tick"`
	Screen          string `json:"screen"`
	Dela            string `json:"dela"`      // highlighted-widget DeLa path (navfp dela=)
	MenuItem        int    `json:"menu_item"` // resolved MenuItem enum
	MenuItemName    string `json:"menu_item_name"`
	GameConnection  int    `json:"game_connection"`  // conn: 0 menu, 1 system-link, 2 hosting, 3 film
	PregameSentinel bool   `json:"pregame_sentinel"` // game_globals+0x10 == 0xDEADBEEF
	MenuFocus       uint32 `json:"menu_focus"`       // 0x2F9B38 focused-widget ptr — 0 on a cold un-woken menu

	MapCursor      CursorView `json:"map_cursor"`
	GametypeCursor CursorView `json:"gametype_cursor"`

	// Map/Gametype are the READ (currently-loaded) values; SelectedMap/Gametype are
	// the player's picked intent off the runner's selector.
	Map              string `json:"map"`
	Gametype         string `json:"gametype"`
	SelectedMap      string `json:"selected_map"`
	SelectedGametype string `json:"selected_gametype"`
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
		d.MapCursor = CursorView{Index: ev.MapCursor, Count: ev.MapCursorCount, Valid: ev.MapCursorValid}
		d.GametypeCursor = CursorView{Index: ev.GametypeCursor, Count: ev.GametypeCursorCount, Valid: ev.GametypeCursorValid}
		d.Map = ev.Map
		d.Gametype = ev.Gametype
	}
	return d
}
