package games

import (
	"sort"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/rating"
	"github.com/Stewball32/xemu-cartographer/internal/series"
)

// stampGameEvents backfills the `game` relation onto an instance's unstamped
// game_events rows that fall inside the finished game's time window (M13
// option-a: events are written instance-keyed by the live capture sink, then
// associated to a game at game-end). Idempotent — only rows with an empty
// `game` are touched. Returns the number stamped.
//
// The window is applied in Go (by `ts`) rather than SQL so the date-format
// matching stays robust; a long-running instance could make this O(all events)
// — a SQL seq/tick range filter is the efficiency follow-up.
func stampGameEvents(app core.App, gameID, instance string, start, end time.Time) (int, error) {
	if gameID == "" || instance == "" {
		return 0, nil
	}
	rows, err := app.FindRecordsByFilter(
		"game_events",
		"instance = {:inst} && game = ''",
		"", 0, 0,
		dbx.Params{"inst": instance},
	)
	if err != nil {
		return 0, err
	}
	windowed := !start.IsZero() || !end.IsZero()
	count := 0
	for _, r := range rows {
		if windowed {
			ts := r.GetDateTime("ts").Time()
			// An event with no timestamp can't be placed in the window — skip
			// it rather than mis-attribute it to this game.
			if ts.IsZero() {
				continue
			}
			if !start.IsZero() && ts.Before(start) {
				continue
			}
			if !end.IsZero() && ts.After(end) {
				continue
			}
		}
		r.Set("game", gameID)
		if err := app.Save(r); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// advanceSeries recomputes a series' standing from its games' winner_team tally
// and, when the format's termination condition is met, stamps `ended_at`
// (unless already set). Returns the computed Standing. A 1-game casual series
// auto-created by createSeriesForGame already has ended_at set, so this is a
// no-op write for it but still returns Complete=true.
func advanceSeries(app core.App, seriesID string, completedAt time.Time) (series.Standing, error) {
	var std series.Standing
	if seriesID == "" {
		return std, nil
	}
	s, err := app.FindRecordById("series", seriesID)
	if err != nil {
		return std, err
	}
	games, err := app.FindRecordsByFilter("games", "series = {:s}", "", 0, 0, dbx.Params{"s": seriesID})
	if err != nil {
		return std, err
	}

	var teamWins []int
	for _, g := range games {
		// Note: an unrecorded winner reads winner_team as 0, so a draw credits
		// team 0 — same known limitation as the M15 Won projection.
		t := g.GetInt("winner_team")
		if t < 0 {
			continue
		}
		for len(teamWins) <= t {
			teamWins = append(teamWins, 0)
		}
		teamWins[t]++
	}

	std = series.Progress(s.GetString("format"), s.GetInt("target_n"), teamWins)
	if std.Complete && s.GetDateTime("ended_at").Time().IsZero() && !completedAt.IsZero() {
		s.Set("ended_at", completedAt)
		if err := app.Save(s); err != nil {
			return std, err
		}
	}
	return std, nil
}

// updateRatings applies a per-game-type Elo update for a finished game and
// upserts each player's rating. v1 handles the two-team case (team-average
// Elo, the delta applied to every player on the team); other team counts (FFA)
// only bump the game count — FFA rating is a documented M18 follow-up. Returns
// the number of rating rows written.
func updateRatings(app core.App, gametype string, players []PlayerStat, winnerTeam *int) (int, error) {
	if gametype == "" || len(players) == 0 {
		return 0, nil
	}

	teams := map[int][]int{}
	for i, p := range players {
		teams[p.Team] = append(teams[p.Team], i)
	}

	cur := make([]float64, len(players))
	gamesPlayed := make([]int, len(players))
	recs := make([]*core.Record, len(players))
	for i, p := range players {
		cur[i], gamesPlayed[i], recs[i] = loadRating(app, p.Gamertag, gametype)
	}

	deltas := make([]float64, len(players))
	if len(teams) == 2 {
		ids := make([]int, 0, 2)
		for id := range teams {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		a, b := ids[0], ids[1]
		avgA := avgRating(cur, teams[a])
		avgB := avgRating(cur, teams[b])

		scoreA := rating.Draw
		if winnerTeam != nil {
			switch *winnerTeam {
			case a:
				scoreA = rating.WinA
			case b:
				scoreA = rating.LossA
			}
		}
		newA, _ := rating.Update(avgA, avgB, scoreA, rating.DefaultK)
		dA := newA - avgA
		for _, i := range teams[a] {
			deltas[i] = dA
		}
		for _, i := range teams[b] {
			deltas[i] = -dA
		}
	}

	count := 0
	for i, p := range players {
		if err := saveRating(app, p.Gamertag, gametype, cur[i]+deltas[i], gamesPlayed[i]+1, recs[i]); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func avgRating(cur []float64, idxs []int) float64 {
	if len(idxs) == 0 {
		return rating.DefaultRating
	}
	sum := 0.0
	for _, i := range idxs {
		sum += cur[i]
	}
	return sum / float64(len(idxs))
}

// loadRating returns a player's current (rating, gamesPlayed, record) for a
// game type. An unrated player returns (DefaultRating, 0, nil).
func loadRating(app core.App, gamertag, gametype string) (float64, int, *core.Record) {
	rec, err := app.FindFirstRecordByFilter(
		"ratings",
		"gamertag = {:g} && gametype = {:t}",
		dbx.Params{"g": gamertag, "t": gametype},
	)
	if err != nil || rec == nil {
		return rating.DefaultRating, 0, nil
	}
	return rec.GetFloat("rating"), rec.GetInt("games"), rec
}

// saveRating upserts a (gamertag, gametype) rating row.
func saveRating(app core.App, gamertag, gametype string, newRating float64, games int, rec *core.Record) error {
	if rec == nil {
		col, err := app.FindCollectionByNameOrId("ratings")
		if err != nil {
			return err
		}
		rec = core.NewRecord(col)
		rec.Set("gamertag", gamertag)
		rec.Set("gametype", gametype)
	}
	rec.Set("rating", newRating)
	rec.Set("games", games)
	return app.Save(rec)
}
