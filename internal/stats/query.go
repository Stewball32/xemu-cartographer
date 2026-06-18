package stats

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// PlayerGamesForGamertag fetches every game_players row for the exact gamertag
// string and projects each into a PlayerGame, joined to its game (gametype +
// winner→Won) and the game's series (category). Newest first.
//
// Note on Won: an unrecorded winner reads winner_team as 0, which a real team-0
// would also read as, so a draw/no-winner game can credit team 0 with a win.
// Acceptable for v1; a `has_winner`/draw flag on games is the follow-up.
func PlayerGamesForGamertag(app core.App, gamertag string) ([]PlayerGame, error) {
	if gamertag == "" {
		return nil, nil
	}
	rows, err := app.FindRecordsByFilter(
		"game_players",
		"gamertag = {:gt}",
		"-created", 0, 0,
		dbx.Params{"gt": gamertag},
	)
	if err != nil {
		return nil, err
	}
	return projectPlayerGames(app, rows)
}

// PlayerGamesForGamertags unions the lines for several gamertags — a user's
// full M7 gamertag set — into one slice ready for Roll / RollByGametype.
func PlayerGamesForGamertags(app core.App, gamertags []string) ([]PlayerGame, error) {
	var all []PlayerGame
	for _, gt := range gamertags {
		lines, err := PlayerGamesForGamertag(app, gt)
		if err != nil {
			return nil, err
		}
		all = append(all, lines...)
	}
	return all, nil
}

func projectPlayerGames(app core.App, rows []*core.Record) ([]PlayerGame, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	// Nested expand: game (for gametype/winner) → series (for category).
	for _, expandErr := range app.ExpandRecords(rows, []string{"game.series"}, nil) {
		if expandErr != nil {
			return nil, expandErr
		}
	}

	out := make([]PlayerGame, 0, len(rows))
	for _, r := range rows {
		pg := PlayerGame{
			Gamertag:    r.GetString("gamertag"),
			Team:        r.GetInt("team"),
			Kills:       r.GetInt("kills"),
			Deaths:      r.GetInt("deaths"),
			Assists:     r.GetInt("assists"),
			Score:       r.GetInt("score"),
			TimeAliveMs: int64(r.GetInt("time_alive_ms")),
			// is_dummy is the M15d after-the-fact flag (not yet on the schema);
			// GetBool reads false until that field lands, so no line is dropped.
			IsDummy: r.GetBool("is_dummy"),
		}
		if game := r.ExpandedOne("game"); game != nil {
			pg.Gametype = game.GetString("gametype")
			pg.Won = pg.Team == game.GetInt("winner_team")
			if series := game.ExpandedOne("series"); series != nil {
				pg.Category = series.GetString("category")
			}
		}
		out = append(out, pg)
	}
	return out, nil
}
