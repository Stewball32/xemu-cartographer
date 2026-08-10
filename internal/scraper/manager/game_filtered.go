package manager

import (
	"time"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/roster"
)

// dummyCfgTTL bounds how stale the cached dummy-filter config may be. Short
// enough that a live is_neutral_host flip (or a dummy_gamertags edit) takes
// effect on viewer overlays within a few seconds; long enough that the DB read
// stays off the 30 Hz hot path.
const dummyCfgTTL = 10 * time.Second

// dummyConfig returns this runner's dummy-filter config, reloading from the DB
// (roster.LoadConfig) when the cache is empty or older than dummyCfgTTL. Reused
// by the game_filtered broadcast (loop goroutine) and join replay (request
// goroutine), so it's mutex-guarded. A nil app (test harness) yields the zero
// Config — a no-op filter.
func (r *runner) dummyConfig(app core.App) roster.Config {
	r.dummyMu.RLock()
	cfg, at := r.dummyCfg, r.dummyCfgAt
	r.dummyMu.RUnlock()
	if !at.IsZero() && time.Since(at) < dummyCfgTTL {
		return cfg
	}
	fresh := roster.LoadConfig(app, r.name)
	r.dummyMu.Lock()
	r.dummyCfg, r.dummyCfgAt = fresh, time.Now()
	r.dummyMu.Unlock()
	return fresh
}

// buildGameFilteredPayload is buildGamePayload with the dummy roster removed —
// identical GamePayload wire shape as the `game` class, minus dummies
// (roster.FilterRoster). It powers the game_filtered class so dummy filtering
// stays SERVER-SIDE for the viewer overlays, while the raw `game` class
// (debug) stays unfiltered. Filtering the cache's GameData.Players before
// buildGamePayload keeps the mapping in one place.
//
// The unified dummy rule: local seats are hidden BY DEFAULT until the match
// accumulator latches them Active (real input/movement — see scraper.MatchAccum);
// is_neutral_host stays as the hard "always hide this host's locals" override;
// the dummy_gamertags allowlist applies to everyone.
func buildGameFilteredPayload(c *instanceCache, cfg roster.Config) GamePayload {
	if c == nil || c.GameData == nil {
		return buildGamePayload(c)
	}
	cfg.HideInactiveLocals = true
	cfg.ActiveLocals = activeLocals(c.PlayerAccum)
	gdCopy := *c.GameData
	gdCopy.Players = roster.FilterRoster(c.GameData.Players, cfg)
	cCopy := *c
	cCopy.GameData = &gdCopy
	return buildGamePayload(&cCopy)
}

// activeLocals projects an accumulator snapshot into the filter's
// index→latched-Active view. Nil snapshot → nil map (nobody active yet, so
// every local seat stays hidden — the pre-match default).
func activeLocals(snap map[int]scraper.PlayerAccum) map[int]bool {
	if len(snap) == 0 {
		return nil
	}
	out := make(map[int]bool, len(snap))
	for idx, st := range snap {
		if st.Active {
			out[idx] = true
		}
	}
	return out
}
