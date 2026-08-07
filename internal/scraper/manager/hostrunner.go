package manager

import (
	"time"

	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/hostrunner"
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// hostTickMinInterval bounds how often the host runner is ticked, decoupling it
// from the scraper's per-phase poll cadence (500ms Ready, 10ms Live). It keeps
// the observable admin stream + input re-evaluation at a few Hz — well below the
// runner's own gating timings (BlindAdvanceAfter 900ms, RepressAfter 1200ms), so
// menu navigation still lands, while the 30Hz Live poll doesn't flood the admin
// room with "match live" waits.
const hostTickMinInterval = 400 * time.Millisecond

// enumMinInterval bounds how often the create-game carousel enumeration runs.
// The lists change only when a disc / gametype set changes, so a few-second
// spacing is ample and keeps the ~1000-entry tag-array walk off the hot path.
const enumMinInterval = 3 * time.Second

// enumerateLobby refreshes the runner's available-maps/gametypes cache from the
// live create-game carousels, feeding the same seam Manager.SetAvailableMaps
// writes (so /api/play/options serves real per-instance lists). No-op when the
// bound GameReader can't enumerate (title without a LobbyEnumerator) or the tick
// throttle hasn't elapsed. On an unavailable read (mid-match, tags not loaded) it
// KEEPS the last successful enumeration rather than clobbering it with empties.
// Loop-goroutine only (reads r.reader directly).
func (r *runner) enumerateLobby() {
	enum, ok := r.reader.(scraper.LobbyEnumerator)
	if !ok {
		return
	}
	now := time.Now()
	if now.Sub(r.lastEnumAt) < enumMinInterval {
		return
	}
	r.lastEnumAt = now

	opts := enum.EnumerateLobby()
	if !opts.Available {
		return
	}
	maps := toMapOptions(opts.Maps)
	gametypes := toMapOptions(opts.Gametypes)
	r.withCache(func(c *instanceCache) {
		c.AvailableMaps = maps
		c.AvailableGametypes = gametypes
	})
}

// toMapOptions maps the game-agnostic scraper.LobbyOption slice onto the
// scraperiface.MapOption the player API serves (identical shape; kept separate so
// game plugins carry no guards-package dependency).
func toMapOptions(in []scraper.LobbyOption) []scraperiface.MapOption {
	out := make([]scraperiface.MapOption, len(in))
	for i, o := range in {
		out[i] = scraperiface.MapOption{Name: o.Name, Steps: o.Steps}
	}
	return out
}

// tickHost drives one step of the player-hosting runner from the scraper loop
// goroutine. It is a no-op when host-running is disabled or the tick throttle
// hasn't elapsed. The Observation is built from LOOP-OWNED state only — the
// reader's LastStateInputs (main_menu / game_connection) and the loop's working
// GameData copy — so it never touches a GameReader from another goroutine (the
// runner's key presses go out asynchronously via the vncinput pump). Called with
// the freshly-read game state + tick from runReady / runLive.
func (r *runner) tickHost(gs scraper.GameState, tick uint32) {
	if r.host == nil {
		return
	}
	now := time.Now()
	if now.Sub(r.lastHostTickAt) < hostTickMinInterval {
		return
	}
	r.lastHostTickAt = now
	obs := r.buildHostReadout(gs, tick).Observation()
	r.host.Tick(obs, now)
}

// buildHostReadout projects the loop's current reader + game-data state into the
// hostrunner.ScraperReadout the adapter maps to an Observation. Loop-goroutine
// only (reads r.reader and r.gameData directly). Fresh is true because it's only
// called right after a successful ReadGameState; a reader read failure skips the
// call entirely, so the runner never acts on a stale frame.
func (r *runner) buildHostReadout(gs scraper.GameState, tick uint32) hostrunner.ScraperReadout {
	ro := hostrunner.ScraperReadout{
		Fresh:     true,
		Tick:      tick,
		GameState: string(gs),
	}
	if r.reader != nil {
		si := r.reader.LastStateInputs()
		ro.MenuActive = stateInputInt(si, "main_menu") != 0
		ro.GameConnection = stateInputInt(si, "game_connection")
		ro.MenuFocus = uint32(stateInputInt(si, "menu_focus"))
	}
	ro.Map = r.gameData.Map
	ro.Gametype = r.gameData.Gametype
	ro.MachineCount = len(r.gameData.Machines)
	ro.PlayerCount = len(r.gameData.Players)
	ro.TeamCount = distinctTeams(r.gameData)
	r.fillLobbyCursor(gs, &ro)
	return ro
}

// fillLobbyCursor projects the reader's LIVE create-game carousel cursors (CE
// widget +0x4C/+0x54) into the readout. Gated to the front-end menu (gs ==
// GameStateMenu) — the SELECT MAP / SELECT GAMETYPE screens are front-end menus in
// BOTH the system-link and split-screen create paths, so this covers the live-nav
// case while keeping the ~2 MiB UI-heap scan off the in-game 30 Hz hot path.
// No-op when the bound reader can't read a cursor (title without a
// LobbyCursorReader). Off the create-game screens the widgets read count 0 →
// *Valid false, so the runner's card steps hold rather than navigating blind.
// Loop-goroutine only (reads r.reader directly).
func (r *runner) fillLobbyCursor(gs scraper.GameState, ro *hostrunner.ScraperReadout) {
	if gs != scraper.GameStateMenu || r.reader == nil {
		return
	}
	cur, ok := r.reader.(scraper.LobbyCursorReader)
	if !ok {
		return
	}
	c := cur.ReadLobbyCursor()
	ro.MapCursor, ro.MapCursorCount, ro.MapCursorValid = c.MapIndex, c.MapCount, c.MapValid
	ro.GametypeCursor, ro.GametypeCursorCount, ro.GametypeCursorValid = c.GametypeIndex, c.GametypeCount, c.GametypeValid
	// Enumerated (built-in) gametype count — the runner subtracts it from the live
	// widget count to find the custom-variant prefix (see gametypeCustomPrefix).
	r.cacheMu.Lock()
	ro.GametypeListLen = len(r.cache.AvailableGametypes)
	r.cacheMu.Unlock()
}

// stateInputInt coerces a StateInputs value (stored as its native memory width —
// uint8/uint16/uint32/…) to an int. Returns 0 for a missing or non-integer key,
// which the adapter treats as the safe menu/no-op default.
func stateInputInt(si scraper.StateInputs, key string) int {
	v, ok := si[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case uint8:
		return int(n)
	case uint16:
		return int(n)
	case uint32:
		return int(n)
	case int:
		return n
	case int8:
		return int(n)
	case int16:
		return int(n)
	case int32:
		return int(n)
	case uint64:
		return int(n)
	case int64:
		return int(n)
	default:
		return 0
	}
}

// distinctTeams estimates the number of teams present for the runner's native
// start gate (2+ teams). Prefers the distinct Team values across the roster;
// falls back to the team-score slot count when the roster is empty (a lobby
// before players are readable). The runner only TIMES the start press on this —
// Halo's own countdown still gates the actual start — so an approximate count is
// safe.
func distinctTeams(gd scraper.GameData) int {
	seen := map[uint32]struct{}{}
	for _, p := range gd.Players {
		seen[p.Team] = struct{}{}
	}
	if len(seen) > 0 {
		return len(seen)
	}
	return len(gd.TeamScores)
}
