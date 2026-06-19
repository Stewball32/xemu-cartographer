package games_test

import (
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	"github.com/Stewball32/xemu-cartographer/internal/games"
)

// ensureCollections registers minimal series / games / game_players collections
// on a fresh TestApp, mirroring the schema package's shapes inline so the test
// doesn't import schema (which would trigger every collection's init()).
func ensureCollections(t *testing.T, app core.App) {
	t.Helper()

	if _, err := app.FindCollectionByNameOrId("series"); err != nil {
		c := core.NewBaseCollection("series")
		c.Fields.Add(
			&core.TextField{Name: "name"},
			&core.SelectField{Name: "format", MaxSelect: 1, Values: []string{"single", "exact-n", "best-of-n", "first-to-x"}},
			&core.NumberField{Name: "target_n", OnlyInt: true},
			&core.SelectField{Name: "category", MaxSelect: 1, Values: []string{"casual", "competitive", "tournament", "custom"}},
			&core.DateField{Name: "started_at"},
			&core.DateField{Name: "ended_at"},
			&core.AutodateField{Name: "created", OnCreate: true},
		)
		if err := app.Save(c); err != nil {
			t.Fatalf("save series: %v", err)
		}
	}

	seriesCol, err := app.FindCollectionByNameOrId("series")
	if err != nil {
		t.Fatalf("lookup series: %v", err)
	}
	if _, err := app.FindCollectionByNameOrId("games"); err != nil {
		c := core.NewBaseCollection("games")
		c.Fields.Add(
			&core.RelationField{Name: "series", Required: true, CollectionId: seriesCol.Id, MaxSelect: 1},
			&core.TextField{Name: "container"},
			&core.TextField{Name: "host_machine_name"},
			&core.TextField{Name: "map"},
			&core.TextField{Name: "gametype"},
			&core.TextField{Name: "variant_name"},
			&core.DateField{Name: "started_at"},
			&core.DateField{Name: "ended_at"},
			&core.NumberField{Name: "winner_team", OnlyInt: true},
			&core.TextField{Name: "score_summary"},
			&core.AutodateField{Name: "created", OnCreate: true},
		)
		if err := app.Save(c); err != nil {
			t.Fatalf("save games: %v", err)
		}
	}

	gamesCol, err := app.FindCollectionByNameOrId("games")
	if err != nil {
		t.Fatalf("lookup games: %v", err)
	}
	if _, err := app.FindCollectionByNameOrId("game_players"); err != nil {
		c := core.NewBaseCollection("game_players")
		c.Fields.Add(
			&core.RelationField{Name: "game", Required: true, CollectionId: gamesCol.Id, MaxSelect: 1},
			&core.TextField{Name: "gamertag", Required: true},
			&core.NumberField{Name: "team", OnlyInt: true},
			&core.NumberField{Name: "kills", OnlyInt: true},
			&core.NumberField{Name: "deaths", OnlyInt: true},
			&core.NumberField{Name: "assists", OnlyInt: true},
			&core.NumberField{Name: "score", OnlyInt: true},
			&core.NumberField{Name: "time_alive_ms", OnlyInt: true},
			&core.AutodateField{Name: "created", OnCreate: true},
		)
		if err := app.Save(c); err != nil {
			t.Fatalf("save game_players: %v", err)
		}
	}

	// game_events: the instance-keyed firehose + the M13 option-a `game` FK.
	if _, err := app.FindCollectionByNameOrId("game_events"); err != nil {
		c := core.NewBaseCollection("game_events")
		c.Fields.Add(
			&core.TextField{Name: "instance", Required: true},
			&core.TextField{Name: "type", Required: true},
			&core.NumberField{Name: "seq", OnlyInt: true},
			&core.NumberField{Name: "tick", OnlyInt: true},
			&core.DateField{Name: "ts"},
			&core.JSONField{Name: "data", MaxSize: 1 << 20},
			&core.RelationField{Name: "game", CollectionId: gamesCol.Id, MaxSelect: 1},
			&core.AutodateField{Name: "created", OnCreate: true},
		)
		if err := app.Save(c); err != nil {
			t.Fatalf("save game_events: %v", err)
		}
	}

	// ratings: per-(gamertag, gametype) Elo store.
	if _, err := app.FindCollectionByNameOrId("ratings"); err != nil {
		c := core.NewBaseCollection("ratings")
		c.Fields.Add(
			&core.TextField{Name: "gamertag", Required: true},
			&core.TextField{Name: "gametype", Required: true},
			&core.NumberField{Name: "rating"},
			&core.NumberField{Name: "games", OnlyInt: true},
			&core.AutodateField{Name: "created", OnCreate: true},
		)
		c.AddIndex("idx_ratings_gamertag_gametype_unique", true, "gamertag, gametype", "")
		if err := app.Save(c); err != nil {
			t.Fatalf("save ratings: %v", err)
		}
	}
}

func sampleGame() games.FinishedGame {
	winner := 0
	return games.FinishedGame{
		Container:       "pod-a",
		HostMachineName: "console-a",
		Map:             "Blood Gulch",
		Gametype:        "ctf",
		VariantName:     "MLG Tournament v7",
		StartedAt:       time.Unix(1_700_000_000, 0),
		EndedAt:         time.Unix(1_700_000_300, 0),
		WinnerTeam:      &winner,
		ScoreSummary:    "3 — 1",
		Players: []games.PlayerStat{
			{Gamertag: "Stewie", Team: 0, Kills: 10, Deaths: 5, Assists: 2, Score: 3},
			{Gamertag: "BlueFox", Team: 1, Kills: 5, Deaths: 10, Assists: 1, Score: 1},
		},
	}
}

func TestPersistFinishedGame_AutoCreatesSeries(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureCollections(t, app)

	res, err := games.PersistFinishedGame(app, sampleGame())
	if err != nil {
		t.Fatalf("PersistFinishedGame: %v", err)
	}
	if !res.CreatedSeries {
		t.Error("expected a series to be auto-created")
	}
	if res.PlayerRowCount != 2 {
		t.Errorf("player rows = %d, want 2", res.PlayerRowCount)
	}

	// "MLG Tournament v7" → competitive via the M13c heuristic.
	s, err := app.FindRecordById("series", res.SeriesID)
	if err != nil {
		t.Fatalf("find series: %v", err)
	}
	if got := s.GetString("category"); got != "competitive" {
		t.Errorf("auto category = %q, want competitive", got)
	}

	gamesRows, err := app.FindAllRecords("games")
	if err != nil {
		t.Fatalf("FindAllRecords games: %v", err)
	}
	if len(gamesRows) != 1 {
		t.Fatalf("games rows = %d, want 1", len(gamesRows))
	}
	if gamesRows[0].GetString("series") != res.SeriesID {
		t.Error("game not linked to the created series")
	}
	if got := gamesRows[0].GetString("map"); got != "Blood Gulch" {
		t.Errorf("game map = %q, want Blood Gulch", got)
	}

	players, err := app.FindAllRecords("game_players")
	if err != nil {
		t.Fatalf("FindAllRecords game_players: %v", err)
	}
	if len(players) != 2 {
		t.Fatalf("game_players rows = %d, want 2", len(players))
	}
	for _, p := range players {
		if p.GetString("game") != res.GameID {
			t.Error("game_player not linked to the game")
		}
	}
}

func TestPersistFinishedGame_UsesExistingSeries(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureCollections(t, app)

	seriesCol, _ := app.FindCollectionByNameOrId("series")
	s := core.NewRecord(seriesCol)
	s.Set("format", "first-to-x")
	s.Set("target_n", 3)
	s.Set("category", "tournament")
	if err := app.Save(s); err != nil {
		t.Fatalf("seed series: %v", err)
	}

	fg := sampleGame()
	fg.SeriesID = s.Id
	res, err := games.PersistFinishedGame(app, fg)
	if err != nil {
		t.Fatalf("PersistFinishedGame: %v", err)
	}
	if res.CreatedSeries {
		t.Error("should not have created a series when SeriesID was provided")
	}
	if res.SeriesID != s.Id {
		t.Errorf("series id = %q, want %q", res.SeriesID, s.Id)
	}

	all, _ := app.FindAllRecords("series")
	if len(all) != 1 {
		t.Errorf("series count = %d, want 1 (no extra series)", len(all))
	}
}

func TestSuggestCategory(t *testing.T) {
	cases := map[string]string{
		"":                  games.CategoryCasual,
		"Slayer":            games.CategoryCasual,
		"CTF":               games.CategoryCasual,
		"MLG Tournament v7": games.CategoryCompetitive,
		"comp 4v4":          games.CategoryCompetitive,
		"HCS Slayer":        games.CategoryCompetitive,
		"Team Slayer Pro":   games.CategoryCompetitive,
		"casual king":       games.CategoryCasual,
	}
	for variant, want := range cases {
		if got := games.SuggestCategory(variant); got != want {
			t.Errorf("SuggestCategory(%q) = %q, want %q", variant, got, want)
		}
	}
}
