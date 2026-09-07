package games

import (
	"fmt"
	"time"

	"github.com/pocketbase/dbx"
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

	// GameUID is the scraper-generated idempotency key (opaque here — ULID/
	// UUIDv7 minted once at capture). Non-empty and already persisted →
	// PersistFinishedGame is a dedupe no-op. Empty (legacy callers) → no
	// dedupe, every call writes a new row.
	GameUID string
	// EndReason carries the scraper's observed match-exit cause
	// ("postgame" | "left_match" | "shutdown"); stored verbatim as an open
	// set — unknown values are the scraper's contract problem, not ours.
	EndReason string
}

// Result reports what PersistFinishedGame created + the downstream chain
// outcome (events stamped, series standing after this game, ratings updated).
type Result struct {
	SeriesID       string
	GameID         string
	PlayerRowCount int
	CreatedSeries  bool

	// Deduped means a games row with this GameUID already existed, nothing
	// was written, and SeriesID/GameID point at the existing row.
	Deduped bool

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
// surfaced so tests catch regressions and the caller can decide how loud to be.
//
// The whole chain runs in one transaction — a mid-chain failure rolls back
// everything (no orphan series, no half-applied Elo), so an at-least-once
// deliverer can safely retry. A non-empty GameUID dedupes redeliveries: an
// existing games row with that uid makes the call a logged no-op. The cheap
// pre-check catches replays; the games.game_uid unique index catches the
// concurrent race (the losing insert's tx fails and rolls back, then finds the
// winner's committed row).
func PersistFinishedGame(app core.App, fg FinishedGame) (Result, error) {
	if r, ok := findByGameUID(app, fg.GameUID); ok {
		return dedupedResult(app, fg.GameUID, r), nil
	}

	var res Result
	err := app.RunInTransaction(func(tx core.App) error {
		var txErr error
		res, txErr = persistFinishedGameTx(tx, fg)
		return txErr
	})
	if err != nil {
		if r, ok := findByGameUID(app, fg.GameUID); ok {
			return dedupedResult(app, fg.GameUID, r), nil
		}
		return Result{}, err
	}
	return res, nil
}

// findByGameUID returns the existing games row for a non-empty uid, if any.
func findByGameUID(app core.App, uid string) (*core.Record, bool) {
	if uid == "" {
		return nil, false
	}
	r, err := app.FindFirstRecordByFilter("games", "game_uid = {:uid}", dbx.Params{"uid": uid})
	if err != nil || r == nil {
		return nil, false
	}
	return r, true
}

func dedupedResult(app core.App, uid string, existing *core.Record) Result {
	app.Logger().Info("games.PersistFinishedGame: duplicate game_uid, skipping",
		"game_uid", uid, "game", existing.Id)
	return Result{
		Deduped:  true,
		GameID:   existing.Id,
		SeriesID: existing.GetString("series"),
	}
}

// persistFinishedGameTx is the six-step chain body, run inside one transaction.
func persistFinishedGameTx(app core.App, fg FinishedGame) (Result, error) {
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
	g.Set("game_uid", fg.GameUID)
	g.Set("end_reason", fg.EndReason)
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
