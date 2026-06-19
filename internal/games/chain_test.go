package games_test

import (
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/types"

	"github.com/Stewball32/xemu-cartographer/internal/games"
)

// gameWindow matches sampleGame()'s StartedAt / EndedAt.
var (
	winStart = time.Unix(1_700_000_000, 0)
	winMid   = time.Unix(1_700_000_100, 0)
	winEnd   = time.Unix(1_700_000_300, 0)
	before   = time.Unix(1_699_999_000, 0)
	after    = time.Unix(1_700_001_000, 0)
)

func insertEvent(t *testing.T, app core.App, instance string, ts time.Time, gameID string) *core.Record {
	t.Helper()
	col, err := app.FindCollectionByNameOrId("game_events")
	if err != nil {
		t.Fatalf("lookup game_events: %v", err)
	}
	r := core.NewRecord(col)
	r.Set("instance", instance)
	r.Set("type", "kill")
	dt, err := types.ParseDateTime(ts)
	if err != nil {
		t.Fatalf("parse ts: %v", err)
	}
	r.Set("ts", dt)
	if gameID != "" {
		r.Set("game", gameID)
	}
	if err := app.Save(r); err != nil {
		t.Fatalf("save event: %v", err)
	}
	return r
}

func reload(t *testing.T, app core.App, id string) *core.Record {
	t.Helper()
	r, err := app.FindRecordById("game_events", id)
	if err != nil {
		t.Fatalf("reload event %s: %v", id, err)
	}
	return r
}

func TestPersistFinishedGame_StampsEventsInWindow(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureCollections(t, app)

	in1 := insertEvent(t, app, "pod-a", winMid, "")   // in window → stamped
	in2 := insertEvent(t, app, "pod-a", winStart, "") // boundary (>= start) → stamped
	pre := insertEvent(t, app, "pod-a", before, "")   // before window → not stamped
	post := insertEvent(t, app, "pod-a", after, "")   // after window → not stamped
	other := insertEvent(t, app, "pod-b", winMid, "") // different instance → not stamped

	res, err := games.PersistFinishedGame(app, sampleGame())
	if err != nil {
		t.Fatalf("PersistFinishedGame: %v", err)
	}
	if res.EventsStamped != 2 {
		t.Fatalf("EventsStamped = %d, want 2", res.EventsStamped)
	}

	if reload(t, app, in1.Id).GetString("game") != res.GameID {
		t.Error("in-window event in1 was not stamped with the game id")
	}
	if reload(t, app, in2.Id).GetString("game") != res.GameID {
		t.Error("boundary event in2 was not stamped")
	}
	for _, id := range []string{pre.Id, post.Id, other.Id} {
		if g := reload(t, app, id).GetString("game"); g != "" {
			t.Errorf("event %s should be unstamped, got game=%q", id, g)
		}
	}

	// Idempotency: a second persist over the same window stamps nothing new
	// (the events now belong to the first game).
	res2, err := games.PersistFinishedGame(app, sampleGame())
	if err != nil {
		t.Fatalf("second PersistFinishedGame: %v", err)
	}
	if res2.EventsStamped != 0 {
		t.Errorf("second persist EventsStamped = %d, want 0 (already stamped)", res2.EventsStamped)
	}
}

func TestPersistFinishedGame_AdvancesSeriesAndRates(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureCollections(t, app)

	// A best-of-3 series (clinch at 2 wins).
	seriesCol, _ := app.FindCollectionByNameOrId("series")
	s := core.NewRecord(seriesCol)
	s.Set("format", "best-of-n")
	s.Set("target_n", 3)
	s.Set("category", "competitive")
	if err := app.Save(s); err != nil {
		t.Fatalf("seed series: %v", err)
	}

	team0 := 0
	mkGame := func() games.FinishedGame {
		fg := sampleGame()
		fg.SeriesID = s.Id
		fg.Gametype = "slayer"
		fg.WinnerTeam = &team0 // team 0 wins both
		fg.Players = []games.PlayerStat{
			{Gamertag: "Stew", Team: 0, Kills: 10, Deaths: 5},
			{Gamertag: "Fox", Team: 1, Kills: 5, Deaths: 10},
		}
		return fg
	}

	// Game 1: 1-0, not complete yet (clinch 2).
	r1, err := games.PersistFinishedGame(app, mkGame())
	if err != nil {
		t.Fatalf("game 1: %v", err)
	}
	if r1.SeriesStanding.Complete {
		t.Error("series should not be complete after 1 of best-of-3")
	}
	if r1.RatingsUpdated != 2 {
		t.Errorf("game 1 RatingsUpdated = %d, want 2", r1.RatingsUpdated)
	}

	// Game 2: 2-0, series clinched by team 0.
	r2, err := games.PersistFinishedGame(app, mkGame())
	if err != nil {
		t.Fatalf("game 2: %v", err)
	}
	if !r2.SeriesStanding.Complete || r2.SeriesStanding.WinnerTeam != 0 {
		t.Errorf("series standing after game 2 = %+v, want complete winner 0", r2.SeriesStanding)
	}

	// Series ended_at stamped on completion.
	sAfter, _ := app.FindRecordById("series", s.Id)
	if sAfter.GetDateTime("ended_at").Time().IsZero() {
		t.Error("completed series should have ended_at stamped")
	}

	// Ratings: winner up, loser down, two games each.
	stew, err := app.FindFirstRecordByFilter("ratings", "gamertag = 'Stew' && gametype = 'slayer'", nil)
	if err != nil {
		t.Fatalf("find Stew rating: %v", err)
	}
	fox, err := app.FindFirstRecordByFilter("ratings", "gamertag = 'Fox' && gametype = 'slayer'", nil)
	if err != nil {
		t.Fatalf("find Fox rating: %v", err)
	}
	if stew.GetFloat("rating") <= 1500 {
		t.Errorf("winner Stew rating = %v, want > 1500", stew.GetFloat("rating"))
	}
	if fox.GetFloat("rating") >= 1500 {
		t.Errorf("loser Fox rating = %v, want < 1500", fox.GetFloat("rating"))
	}
	if stew.GetInt("games") != 2 || fox.GetInt("games") != 2 {
		t.Errorf("games played = Stew %d / Fox %d, want 2 / 2", stew.GetInt("games"), fox.GetInt("games"))
	}
	// Elo is zero-sum: the two ratings stay centered on the start average.
	if g := stew.GetFloat("rating") + fox.GetFloat("rating"); g < 2999.9 || g > 3000.1 {
		t.Errorf("rating sum = %v, want ~3000 (zero-sum)", g)
	}
}
