package hostrunner

// RunnerEvent is one entry on the observable input stream (requirement 5): the
// high-level player INTENT plus the actual KEY the runner emitted, along with
// the read-only native start conditions. Broadcast to the admin WS room so the
// kiosk can show what "players" are sending.
type RunnerEvent struct {
	Instance  string `json:"instance"`
	Tick      uint32 `json:"tick"`
	Authority string `json:"authority"`
	Screen    string `json:"screen"`

	// Kind mirrors Action.Kind ("tap"/"chord"/"wait"/"blocked"/"done"), plus
	// "suspended" when the runner stood down for an admin.
	Kind   string   `json:"kind"`
	Intent string   `json:"intent,omitempty"`
	Keys   []string `json:"keys,omitempty"`
	Reason string   `json:"reason,omitempty"`

	// Map / Gametype are the READ (currently-loaded) values from the observation
	// — what the admin stream shows the box is actually on, distinct from the
	// player's selected intent (which the play API reads off the runner).
	Map      string `json:"map,omitempty"`
	Gametype string `json:"gametype,omitempty"`

	// Native start conditions — READ, never controlled — always surfaced so the
	// UI can show why start is (not) happening.
	MachineCount    int  `json:"machine_count"`
	TeamCount       int  `json:"team_count"`
	PlayerCount     int  `json:"player_count"`
	CountdownActive bool `json:"countdown_active"`
	ReadyToStart    bool `json:"ready_to_start"`

	// Raw diagnostic reads behind the classified Screen — surfaced for the admin
	// diagnostics panel so an operator can watch the box AND its live scraper reads
	// side-by-side. NOT part of the decision-change log gate (see decisionChanged),
	// so they update every tick without flooding the server log.
	Dela            string `json:"dela,omitempty"`
	MenuItem        int    `json:"menu_item"`
	GameConnection  int    `json:"game_connection"`
	PregameSentinel bool   `json:"pregame_sentinel"`
	// MenuFocus ← menu_focus (0x2F9B38): the CE front-end focused-widget pointer.
	// Candidate "menu woken / widget tree built?" signal — non-zero once the tree is
	// instantiated, expected 0 on a COLD un-interacted main menu (the state where dela
	// is blank). Surfaced so Stewart can verify what populates cold.
	MenuFocus           uint32 `json:"menu_focus"`
	MapCursor           int    `json:"map_cursor"`
	MapCursorCount      int    `json:"map_cursor_count"`
	MapCursorValid      bool   `json:"map_cursor_valid"`
	GametypeCursor      int    `json:"gametype_cursor"`
	GametypeCursorCount int    `json:"gametype_cursor_count"`
	GametypeCursorValid bool   `json:"gametype_cursor_valid"`
}

// EventSink receives the observable stream. The prod adapter serialises to JSON
// and broadcasts to the admin room via svc.WS.SendToRoomRaw; tests use a fake.
type EventSink interface {
	Emit(RunnerEvent)
}

// NopSink discards events (used when no observer is wired).
type NopSink struct{}

func (NopSink) Emit(RunnerEvent) {}

// buildEvent stamps an Action + observation into a RunnerEvent.
func buildEvent(instance string, obs Observation, auth Authority, kind string, act Action) RunnerEvent {
	return RunnerEvent{
		Instance:        instance,
		Tick:            obs.Tick,
		Authority:       auth.String(),
		Screen:          Classify(obs).String(),
		Kind:            kind,
		Intent:          act.Intent,
		Keys:            act.Keys,
		Reason:          act.Reason,
		Map:             obs.Map,
		Gametype:        obs.Gametype,
		MachineCount:    obs.MachineCount,
		TeamCount:       obs.TeamCount,
		PlayerCount:     obs.PlayerCount,
		CountdownActive: obs.CountdownActive,
		ReadyToStart:    obs.ReadyToStart(),

		Dela:                obs.Dela,
		MenuItem:            int(obs.MenuItem),
		GameConnection:      int(obs.Connection),
		PregameSentinel:     obs.PregameSentinel,
		MenuFocus:           obs.MenuFocus,
		MapCursor:           obs.MapCursor,
		MapCursorCount:      obs.MapCursorCount,
		MapCursorValid:      obs.MapCursorValid,
		GametypeCursor:      obs.GametypeCursor,
		GametypeCursorCount: obs.GametypeCursorCount,
		GametypeCursorValid: obs.GametypeCursorValid,
	}
}
