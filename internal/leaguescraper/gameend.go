package leaguescraper

import (
	"log"

	"github.com/pocketbase/pocketbase/core"

	"github.com/xemu-cartographer/xc-scraper/runner"
	"github.com/xemu-cartographer/xc-scraper/wire"
	"github.com/xemu-cartographer/xemu-cartographer/internal/games"
)

// GameEndHook returns the league server's runner.GameEnd: it adapts the
// scraper's wire.FinishedGame artifact into the league's own
// games.FinishedGame and runs the M13 persistence chain
// (internal/games.PersistFinishedGame → games + game_players + game_events
// stamping + series advance + rating update).
//
// Best-effort: the manager already runs the hook on its own goroutine and
// flushes it on Stop; errors are logged, never surfaced — a persistence
// hiccup must never reach the scraper loop. Idempotent on fg.GameUID (the
// persistence layer dedupes), so at-least-once delivery is safe.
func GameEndHook(app core.App) runner.GameEnd {
	return func(fg wire.FinishedGame) {
		if _, err := games.PersistFinishedGame(app, FinishedGameFromWire(fg)); err != nil {
			log.Printf("leaguescraper: persist finished game (instance=%s): %v", fg.Instance, err)
		}
	}
}

// FinishedGameFromWire maps the wire artifact onto the league persistence
// input. Instance → Container; the engine player name → PlayerStat.Gamertag
// (account mapping happens downstream); per-player counters widen to int.
// StartedAt stays zero (the artifact's started_at is a known gap — the
// event-stamping window then relies on idempotency, see games.FinishedGame)
// and SeriesID empty (auto-created by the chain).
func FinishedGameFromWire(fg wire.FinishedGame) games.FinishedGame {
	out := games.FinishedGame{
		Container:       fg.Instance,
		HostMachineName: fg.HostMachineName,
		Map:             fg.Map,
		Gametype:        fg.Gametype,
		VariantName:     fg.VariantName,
		EndedAt:         fg.EndedAt,
		ScoreSummary:    fg.ScoreSummary,
		GameUID:         fg.GameUID,
		EndReason:       fg.EndReason,
	}
	if fg.WinnerTeam != nil {
		w := int(*fg.WinnerTeam)
		out.WinnerTeam = &w
	}
	for _, p := range fg.Players {
		out.Players = append(out.Players, games.PlayerStat{
			Gamertag: p.Name,
			Team:     int(p.Team),
			Kills:    int(p.Kills),
			Deaths:   int(p.Deaths),
			Assists:  int(p.Assists),
			Score:    int(p.Score),
		})
	}
	return out
}
