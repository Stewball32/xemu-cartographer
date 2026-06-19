package games

import (
	"fmt"
	"time"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/series"
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

// Result reports what PersistFinishedGame created + the downstream chain
// outcome (events stamped, series standing after this game, ratings updated).
type Result struct {
	SeriesID       string
	GameID         string
	PlayerRowCount int
	CreatedSeries  bool

	// EventsStamped is how many game_events rows were back-linked to this game
	// (M13 option-a). SeriesStanding is the series' standing after this game
	// (M14). RatingsUpdated is how many ratings rows were upserted (M18).
	EventsStamped  int
	SeriesStanding series.Standing
	RatingsUpdated int
}

// PersistFinishedGame writes one `games` row + N `game_players` rows for a
// finished contest, auto-creating a 1-game series when SeriesID is empty, then
// runs the game-end chain: stamp this instance's in-window `game_events` rows
// with the game id (M13 option-a), advance the series standing (M14
// series.Progress, ending it when the format is satisfied), and apply the
// per-game-type Elo rating update (M18). Returns the created ids + chain
// outcome.
//
// The caller (the scraper Live→Ready hook) is best-effort: log + continue on
// error so a persistence hiccup never stalls the scraper loop. Errors are
// surfaced (with the game id already in Result) so tests catch regressions and
// the caller can decide how loud to be.
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

	// Game-end chain: stamp events → advance series → update ratings.
	stamped, err := stampGameEvents(app, g.Id, fg.Container, fg.StartedAt, fg.EndedAt)
	if err != nil {
		return res, fmt.Errorf("games.PersistFinishedGame: stamp events: %w", err)
	}
	res.EventsStamped = stamped

	std, err := advanceSeries(app, seriesID, fg.EndedAt)
	if err != nil {
		return res, fmt.Errorf("games.PersistFinishedGame: advance series: %w", err)
	}
	res.SeriesStanding = std

	rated, err := updateRatings(app, fg.Gametype, fg.Players, fg.WinnerTeam)
	if err != nil {
		return res, fmt.Errorf("games.PersistFinishedGame: update ratings: %w", err)
	}
	res.RatingsUpdated = rated

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
