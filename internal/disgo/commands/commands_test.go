package commands

import (
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	"github.com/Stewball32/xemu-cartographer/internal/discordcfg"
	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
)

func TestParseCategories(t *testing.T) {
	got := parseCategories("casual, Competitive , bogus,tournament,,CUSTOM")
	want := []string{"casual", "competitive", "tournament", "custom"}
	if len(got) != len(want) {
		t.Fatalf("parseCategories = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("category %d = %q, want %q", i, got[i], want[i])
		}
	}
	if parseCategories("") != nil {
		t.Error("empty CSV should parse to nil")
	}
}

func TestStatusEmbed(t *testing.T) {
	if statusEmbed(nil).Description == "" {
		t.Error("empty membership → description set")
	}
	view := []scraperiface.ContainerMembership{
		{Container: "pod-a", Identities: []string{"stewie", "bluefox"}},
		{Container: "pod-b", Identities: []string{"red"}},
	}
	e := statusEmbed(view)
	if len(e.Fields) != 2 {
		t.Fatalf("status fields = %d, want 2", len(e.Fields))
	}
	if e.Fields[0].Name != "pod-a" || !strings.Contains(e.Fields[0].Value, "2") {
		t.Errorf("pod-a field = %+v, want 2 on roster", e.Fields[0])
	}
}

func ensureStatsCollections(t *testing.T, app core.App) {
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
			&core.AutodateField{Name: "created", OnCreate: true},
		)
		if err := app.Save(c); err != nil {
			t.Fatalf("save game_players: %v", err)
		}
	}
}

func mkRec(t *testing.T, app core.App, col string, set map[string]any) *core.Record {
	t.Helper()
	c, _ := app.FindCollectionByNameOrId(col)
	r := core.NewRecord(c)
	for k, v := range set {
		r.Set(k, v)
	}
	if err := app.Save(r); err != nil {
		t.Fatalf("save %s: %v", col, err)
	}
	return r
}

func TestUserStatsEmbed(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureStatsCollections(t, app)

	s := mkRec(t, app, "series", map[string]any{"category": "competitive"})
	g := mkRec(t, app, "games", map[string]any{"series": s.Id, "gametype": "slayer", "winner_team": 0})
	mkRec(t, app, "game_players", map[string]any{"game": g.Id, "gamertag": "Stew", "team": 0, "kills": 10, "deaths": 5})

	e := userStatsEmbed(app, "Stew")
	if !strings.Contains(e.Title, "Stew") {
		t.Errorf("title = %q, want gamertag", e.Title)
	}
	if e.Fields[0].Name != "Games" || e.Fields[0].Value != "1" {
		t.Errorf("Games field = %+v, want 1", e.Fields[0])
	}
}

func TestApplyConfig(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	// discord_guilds collection.
	c := core.NewBaseCollection("discord_guilds")
	c.Fields.Add(
		&core.TextField{Name: "guild_id", Required: true},
		&core.TextField{Name: "results_channel"},
		&core.TextField{Name: "tournament_channel"},
		&core.SelectField{Name: "posted_categories", MaxSelect: 4, Values: []string{"casual", "competitive", "tournament", "custom"}},
		&core.AutodateField{Name: "created", OnCreate: true},
	)
	c.AddIndex("idx_dg_unique_t", true, "guild_id", "")
	if err := app.Save(c); err != nil {
		t.Fatalf("save discord_guilds: %v", err)
	}

	if err := applyConfig(app, "guild-9", "chan-9", "casual, competitive, bogus"); err != nil {
		t.Fatalf("applyConfig: %v", err)
	}
	cfg, ok := discordcfg.Get(app, "guild-9")
	if !ok || cfg.ResultsChannel != "chan-9" {
		t.Fatalf("Get = %+v ok=%v", cfg, ok)
	}
	if len(cfg.PostedCategories) != 2 {
		t.Errorf("categories = %v, want [casual competitive] (bogus dropped)", cfg.PostedCategories)
	}
}
