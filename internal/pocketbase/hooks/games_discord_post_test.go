package hooks

import (
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func ensureGameCollections(t *testing.T, app core.App) {
	t.Helper()

	if _, err := app.FindCollectionByNameOrId("series"); err != nil {
		c := core.NewBaseCollection("series")
		c.Fields.Add(
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
			&core.RelationField{Name: "series", CollectionId: seriesCol.Id, MaxSelect: 1},
			&core.TextField{Name: "map"},
			&core.TextField{Name: "gametype"},
			&core.TextField{Name: "variant_name"},
			&core.TextField{Name: "score_summary"},
			&core.NumberField{Name: "winner_team", OnlyInt: true},
			&core.AutodateField{Name: "created", OnCreate: true},
		)
		if err := app.Save(c); err != nil {
			t.Fatalf("save games: %v", err)
		}
	}

	if _, err := app.FindCollectionByNameOrId("discord_guilds"); err != nil {
		c := core.NewBaseCollection("discord_guilds")
		c.Fields.Add(
			&core.TextField{Name: "guild_id", Required: true},
			&core.TextField{Name: "results_channel"},
			&core.TextField{Name: "tournament_channel"},
			&core.SelectField{Name: "posted_categories", MaxSelect: 4, Values: []string{"casual", "competitive", "tournament", "custom"}},
			&core.AutodateField{Name: "created", OnCreate: true},
		)
		c.AddIndex("idx_dg_guild_id_unique_t", true, "guild_id", "")
		if err := app.Save(c); err != nil {
			t.Fatalf("save discord_guilds: %v", err)
		}
	}
}

func saveRec(t *testing.T, app core.App, col string, set map[string]any) *core.Record {
	t.Helper()
	c, err := app.FindCollectionByNameOrId(col)
	if err != nil {
		t.Fatalf("collection %s: %v", col, err)
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

func TestGameResultTargets_CategoryFilter(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureGameCollections(t, app)

	series := saveRec(t, app, "series", map[string]any{"category": "competitive"})
	game := saveRec(t, app, "games", map[string]any{
		"series": series.Id, "map": "Blood Gulch", "gametype": "ctf",
		"score_summary": "3 – 1", "winner_team": 0,
	})

	// One guild posts competitive → should receive it; one posts only casual.
	saveRec(t, app, "discord_guilds", map[string]any{
		"guild_id": "g-comp", "results_channel": "111", "posted_categories": []string{"competitive"},
	})
	saveRec(t, app, "discord_guilds", map[string]any{
		"guild_id": "g-casual", "results_channel": "222", "posted_categories": []string{"casual"},
	})

	embed, targets := gameResultTargets(app, game)

	if len(targets) != 1 || targets[0] != "111" {
		t.Errorf("targets = %v, want [111] (competitive guild only)", targets)
	}
	if !strings.Contains(embed.Title, "Blood Gulch") {
		t.Errorf("embed title = %q, want the map name", embed.Title)
	}
}

func TestGameResultTargets_NoConfiguredGuilds(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureGameCollections(t, app)

	series := saveRec(t, app, "series", map[string]any{"category": "casual"})
	game := saveRec(t, app, "games", map[string]any{"series": series.Id, "gametype": "slayer"})

	_, targets := gameResultTargets(app, game)
	if len(targets) != 0 {
		t.Errorf("no configured guilds → targets = %v, want none", targets)
	}
}
