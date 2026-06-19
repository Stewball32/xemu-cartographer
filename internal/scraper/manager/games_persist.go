package manager

import (
	"log"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/games"
	"github.com/Stewball32/xemu-cartographer/internal/guards"
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// persistFinishedGame is the M13 production trigger: on the Live→Ready edge it
// hands the just-captured game to the PB persistence chain
// (internal/games.PersistFinishedGame → games + game_players + game_events
// stamping + series advance + rating update). Deferred in runLive AFTER
// captureLiveAsPrevious (LIFO), so cache.PreviousGame is populated by the time
// it runs.
//
// Best-effort: the actual write happens on its own goroutine and errors are
// logged, never surfaced — a persistence hiccup must never stall the scraper
// loop. No-op on a nil svc/App (test harnesses) or an empty game.
//
// LIVE GAP: this path can only be verified end-to-end against a real game
// (no xemu here). The mapping below (finishedGameFromGameData) and the
// goroutine are exercised indirectly — internal/games is fully unit-tested
// against the same FinishedGame shape — but the GameData→FinishedGame
// projection itself awaits a live Halo: CE match.
func (r *runner) persistFinishedGame(svc *guards.Services) {
	if svc == nil || svc.App == nil {
		return
	}

	var (
		fg games.FinishedGame
		ok bool
	)
	r.cacheMu.Lock()
	if r.cache.PreviousGame != nil {
		fg, ok = finishedGameFromGameData(r.name, r.cache.PreviousGame.GameData, r.cache.PreviousGame.EndedAt)
	}
	r.cacheMu.Unlock()

	if !ok || len(fg.Players) == 0 {
		return
	}

	go func() {
		if _, err := games.PersistFinishedGame(svc.App, fg); err != nil {
			log.Printf("manager: persist finished game (instance=%s): %v", r.name, err)
		}
	}()
}

// finishedGameFromGameData projects a captured scraper.GameData into the
// games.FinishedGame persistence input. Returns ok=false when there's nothing
// worth persisting (nil data).
//
// StartedAt is intentionally left zero: the event-stamping window then relies
// on idempotency (only unstamped game_events rows are touched, and prior games'
// rows are already stamped), so the just-ended game claims exactly its own
// still-unstamped events. A per-game start timestamp (to also exclude
// idle/menu events between games) is a follow-up.
func finishedGameFromGameData(name string, gd *scraper.GameData, endedAt time.Time) (games.FinishedGame, bool) {
	if gd == nil {
		return games.FinishedGame{}, false
	}

	fg := games.FinishedGame{
		Container:    name,
		Map:          gd.Map,
		Gametype:     gd.Gametype,
		VariantName:  gd.VariantName,
		EndedAt:      endedAt,
		ScoreSummary: renderScoreSummary(gd),
	}

	// Host machine name: the local machine in the system-link roster.
	for _, m := range gd.Machines {
		if m.IsLocal != nil && *m.IsLocal {
			fg.HostMachineName = m.Name
			break
		}
	}

	// Winner: the unique highest-scoring team (team games only). A tie or a
	// non-team game leaves WinnerTeam nil (no winner recorded).
	if gd.IsTeamGame && len(gd.TeamScores) > 0 {
		best, bestScore, tie := -1, int32(0), false
		for i, ts := range gd.TeamScores {
			if i == 0 || ts.Score > bestScore {
				best, bestScore, tie = int(ts.Team), ts.Score, false
			} else if ts.Score == bestScore {
				tie = true
			}
		}
		if best >= 0 && !tie {
			w := best
			fg.WinnerTeam = &w
		}
	}

	for _, p := range gd.Players {
		fg.Players = append(fg.Players, games.PlayerStat{
			Gamertag: p.Name,
			Team:     int(p.Team),
			Kills:    int(p.Kills),
			Deaths:   int(p.Deaths),
			Assists:  int(p.Assists),
			Score:    int(p.Score),
		})
	}

	return fg, true
}
