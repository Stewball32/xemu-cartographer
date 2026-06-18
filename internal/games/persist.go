package games

import (
	"fmt"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// PlayerStat is one player's per-game line for a finished contest.
type PlayerStat struct {
	Gamertag    string
	Team        int
	Kills       int
	Deaths      int
	Assists     int
	Score       int
	TimeAliveMs int
}

// FinishedGame is the persistence input for one completed contest, assembled
// from the scraper's Live→Ready `cache.PreviousGame` data + the owning
// container. An empty SeriesID auto-creates a 1-game series categorized from
// VariantName (M13b: "if no series exists, create a 1-game series").
type FinishedGame struct {
	SeriesID        string
	Container       string
	HostMachineName string
	Map             string
	Gametype        string
	VariantName     string
	StartedAt       time.Time
	EndedAt         time.Time
	WinnerTeam      *int
	ScoreSummary    string
	Players         []PlayerStat
}

// Result reports what PersistFinishedGame created.
type Result struct {
	SeriesID       string
	GameID         string
	PlayerRowCount int
	CreatedSeries  bool
}

// PersistFinishedGame writes one `games` row + N `game_players` rows for a
// finished contest, auto-creating a 1-game series when SeriesID is empty.
// Returns the created/linked ids. The caller (the scraper Live→Ready hook) is
// best-effort: log + continue on error so a persistence hiccup never stalls
// the scraper loop. Per-tick events stay in the existing instance-keyed
// game_events firehose; linking them to a game is a deferred M13 decision.
func PersistFinishedGame(app core.App, fg FinishedGame) (Result, error) {
	var res Result

	seriesID := fg.SeriesID
	if seriesID == "" {
		sid, err := createSeriesForGame(app, fg)
		if err != nil {
			return res, fmt.Errorf("games.PersistFinishedGame: create series: %w", err)
		}
		seriesID = sid
		res.CreatedSeries = true
	}
	res.SeriesID = seriesID

	gamesCol, err := app.FindCollectionByNameOrId("games")
	if err != nil {
		return res, fmt.Errorf("games.PersistFinishedGame: lookup games: %w", err)
	}
	g := core.NewRecord(gamesCol)
	g.Set("series", seriesID)
	g.Set("container", fg.Container)
	g.Set("host_machine_name", fg.HostMachineName)
	g.Set("map", fg.Map)
	g.Set("gametype", fg.Gametype)
	g.Set("variant_name", fg.VariantName)
	if !fg.StartedAt.IsZero() {
		g.Set("started_at", fg.StartedAt)
	}
	if !fg.EndedAt.IsZero() {
		g.Set("ended_at", fg.EndedAt)
	}
	if fg.WinnerTeam != nil {
		g.Set("winner_team", *fg.WinnerTeam)
	}
	g.Set("score_summary", fg.ScoreSummary)
	if err := app.Save(g); err != nil {
		return res, fmt.Errorf("games.PersistFinishedGame: save game: %w", err)
	}
	res.GameID = g.Id

	playersCol, err := app.FindCollectionByNameOrId("game_players")
	if err != nil {
		return res, fmt.Errorf("games.PersistFinishedGame: lookup game_players: %w", err)
	}
	for _, p := range fg.Players {
		pr := core.NewRecord(playersCol)
		pr.Set("game", g.Id)
		pr.Set("gamertag", p.Gamertag)
		pr.Set("team", p.Team)
		pr.Set("kills", p.Kills)
		pr.Set("deaths", p.Deaths)
		pr.Set("assists", p.Assists)
		pr.Set("score", p.Score)
		pr.Set("time_alive_ms", p.TimeAliveMs)
		if err := app.Save(pr); err != nil {
			return res, fmt.Errorf("games.PersistFinishedGame: save game_player %q: %w", p.Gamertag, err)
		}
		res.PlayerRowCount++
	}

	return res, nil
}

// createSeriesForGame creates the implicit 1-game series for a pickup match.
// Category is the M13c heuristic suggestion; format is exact-1; ended_at is
// stamped because the game (and therefore the series) just finished.
func createSeriesForGame(app core.App, fg FinishedGame) (string, error) {
	col, err := app.FindCollectionByNameOrId("series")
	if err != nil {
		return "", err
	}
	s := core.NewRecord(col)
	s.Set("format", "exact-n")
	s.Set("target_n", 1)
	s.Set("category", SuggestCategory(fg.VariantName))
	if !fg.StartedAt.IsZero() {
		s.Set("started_at", fg.StartedAt)
	}
	if !fg.EndedAt.IsZero() {
		s.Set("ended_at", fg.EndedAt)
	}
	if err := app.Save(s); err != nil {
		return "", err
	}
	return s.Id, nil
}
