package manager

import (
	"time"

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
	}
	ro.Map = r.gameData.Map
	ro.Gametype = r.gameData.Gametype
	ro.MachineCount = len(r.gameData.Machines)
	ro.PlayerCount = len(r.gameData.Players)
	ro.TeamCount = distinctTeams(r.gameData)
	return ro
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
