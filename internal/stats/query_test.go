package stats_test

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	"github.com/Stewball32/xemu-cartographer/internal/stats"
)

// ensureCollections registers minimal series / games / game_players collections
// on a fresh TestApp (same inline shapes the schema package uses).
func ensureCollections(t *testing.T, app core.App) {
	t.Helper()

	if _, err := app.FindCollectionByNameOrId("series"); err != nil {
		c := core.NewBaseCollection("series")
		c.Fields.Add(
			&core.SelectField{Name: "format", MaxSelect: 1, Values: []string{"single", "exact-n", "best-of-n", "first-to-x"}},
			&core.NumberField{Name: "target_n", OnlyInt: true},
			&core.SelectField{Name: "category", MaxSelect: 1, Values: []string{"casual", "competitive", "tournament", "custom"}},
			&core.AutodateField{Name: "created", OnCreate: true},
		)
		if err := app.Save(c); err != nil {
			t.Fatalf("save series: %v", err)
		}
	}
	seriesCol, _ := app.FindCollectionByNameOrId("series")

	if _, err := app.FindCollectionByNameOrId("games"); err != nil {
		c := core.NewBaseCollection("games")
		c.Fields.Add(
			&core.RelationField{Name: "series", Required: true, CollectionId: seriesCol.Id, MaxSelect: 1},
			&core.TextField{Name: "gametype"},
			&core.NumberField{Name: "winner_team", OnlyInt: true},
			&core.AutodateField{Name: "created", OnCreate: true},
		)
		if err := app.Save(c); err != nil {
			t.Fatalf("save games: %v", err)
		}
	}
	gamesCol, _ := app.FindCollectionByNameOrId("games")

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
}

func mustSave(t *testing.T, app core.App, col string, set map[string]any) *core.Record {
	t.Helper()
	c, err := app.FindCollectionByNameOrId(col)
	if err != nil {
		t.Fatalf("lookup %s: %v", col, err)
	}
	r := core.NewRecord(c)
	for k, v := range set {
		r.Set(k, v)
	}
	if err := app.Save(r); err != nil {
		t.Fatalf("save %s: %v", col, err)
	}
	return r
}

func TestPlayerGamesForGamertag(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureCollections(t, app)

	s := mustSave(t, app, "series", map[string]any{"format": "single", "target_n": 1, "category": "competitive"})
	g := mustSave(t, app, "games", map[string]any{"series": s.Id, "gametype": "slayer", "winner_team": 0})
	// Stew (team 0) won; Fox (team 1) lost.
	mustSave(t, app, "game_players", map[string]any{"game": g.Id, "gamertag": "Stew", "team": 0, "kills": 10, "deaths": 5})
	mustSave(t, app, "game_players", map[string]any{"game": g.Id, "gamertag": "Fox", "team": 1, "kills": 5, "deaths": 10})

	stewLines, err := stats.PlayerGamesForGamertag(app, "Stew")
	if err != nil {
		t.Fatalf("PlayerGamesForGamertag(Stew): %v", err)
	}
	if len(stewLines) != 1 {
		t.Fatalf("Stew lines = %d, want 1", len(stewLines))
	}
	l := stewLines[0]
	if l.Gametype != "slayer" {
		t.Errorf("gametype = %q, want slayer", l.Gametype)
	}
	if l.Category != "competitive" {
		t.Errorf("category = %q, want competitive (joined via game→series)", l.Category)
	}
	if !l.Won {
		t.Error("Stew (team 0) should have Won (winner_team 0)")
	}
	if l.Kills != 10 || l.Deaths != 5 {
		t.Errorf("K/D = %d/%d, want 10/5", l.Kills, l.Deaths)
	}

	foxLines, err := stats.PlayerGamesForGamertag(app, "Fox")
	if err != nil {
		t.Fatalf("PlayerGamesForGamertag(Fox): %v", err)
	}
	if len(foxLines) != 1 || foxLines[0].Won {
		t.Errorf("Fox should have one losing line; got %d lines won=%v", len(foxLines), len(foxLines) > 0 && foxLines[0].Won)
	}

	// Roll across the (single-gamertag) user view.
	tot := stats.Roll(stewLines)
	if tot.Games != 1 || tot.Wins != 1 || tot.Kills != 10 {
		t.Errorf("Roll = %+v, want games1 wins1 kills10", tot)
	}
}
