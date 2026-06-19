package discordcfg_test

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	"github.com/Stewball32/xemu-cartographer/internal/discordcfg"
)

func TestResultsTarget(t *testing.T) {
	g := discordcfg.GuildConfig{
		GuildID:          "g1",
		ResultsChannel:   "chan-results",
		PostedCategories: []string{"competitive", "tournament"},
	}
	if got := g.ResultsTarget("competitive"); got != "chan-results" {
		t.Errorf("opted-in category = %q, want chan-results", got)
	}
	if got := g.ResultsTarget("casual"); got != "" {
		t.Errorf("non-opted category = %q, want '' (filtered)", got)
	}

	noChannel := discordcfg.GuildConfig{GuildID: "g2", PostedCategories: []string{"casual"}}
	if got := noChannel.ResultsTarget("casual"); got != "" {
		t.Errorf("no results channel = %q, want ''", got)
	}

	empty := discordcfg.GuildConfig{GuildID: "g3", ResultsChannel: "c"}
	if got := empty.ResultsTarget("casual"); got != "" {
		t.Errorf("empty allowlist = %q, want '' (opt-in, posts nothing)", got)
	}
}

func TestResultsTargets_FanOut(t *testing.T) {
	configs := []discordcfg.GuildConfig{
		{GuildID: "a", ResultsChannel: "ca", PostedCategories: []string{"competitive"}},
		{GuildID: "b", ResultsChannel: "cb", PostedCategories: []string{"casual", "competitive"}},
		{GuildID: "c", ResultsChannel: "cc", PostedCategories: []string{"casual"}}, // tournament-only guild won't get competitive
		{GuildID: "d", ResultsChannel: "", PostedCategories: []string{"competitive"}},
	}
	got := discordcfg.ResultsTargets(configs, "competitive")
	want := map[string]bool{"ca": true, "cb": true}
	if len(got) != 2 {
		t.Fatalf("competitive targets = %v, want [ca cb]", got)
	}
	for _, ch := range got {
		if !want[ch] {
			t.Errorf("unexpected target %q", ch)
		}
	}

	// A casual game only reaches guilds that opted into casual.
	casual := discordcfg.ResultsTargets(configs, "casual")
	if len(casual) != 2 { // cb, cc
		t.Errorf("casual targets = %v, want 2 (cb, cc)", casual)
	}
}

func ensureCollection(t *testing.T, app core.App) {
	t.Helper()
	if _, err := app.FindCollectionByNameOrId("discord_guilds"); err == nil {
		return
	}
	c := core.NewBaseCollection("discord_guilds")
	c.Fields.Add(
		&core.TextField{Name: "guild_id", Required: true},
		&core.TextField{Name: "results_channel"},
		&core.TextField{Name: "tournament_channel"},
		&core.SelectField{Name: "posted_categories", MaxSelect: 4, Values: []string{"casual", "competitive", "tournament", "custom"}},
		&core.AutodateField{Name: "created", OnCreate: true},
	)
	c.AddIndex("idx_discord_guilds_guild_id_unique_t", true, "guild_id", "")
	if err := app.Save(c); err != nil {
		t.Fatalf("save discord_guilds: %v", err)
	}
}

func TestUpsertGetAll(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureCollection(t, app)

	if err := discordcfg.Upsert(app, discordcfg.GuildConfig{
		GuildID:          "guild-1",
		ResultsChannel:   "chan-1",
		PostedCategories: []string{"competitive"},
	}); err != nil {
		t.Fatalf("Upsert (create): %v", err)
	}

	got, ok := discordcfg.Get(app, "guild-1")
	if !ok || got.ResultsChannel != "chan-1" || len(got.PostedCategories) != 1 {
		t.Fatalf("Get after create = %+v, ok=%v", got, ok)
	}

	// Upsert again updates in place (no duplicate row).
	if err := discordcfg.Upsert(app, discordcfg.GuildConfig{
		GuildID:          "guild-1",
		ResultsChannel:   "chan-2",
		PostedCategories: []string{"casual", "competitive"},
	}); err != nil {
		t.Fatalf("Upsert (update): %v", err)
	}
	got, _ = discordcfg.Get(app, "guild-1")
	if got.ResultsChannel != "chan-2" || len(got.PostedCategories) != 2 {
		t.Errorf("Get after update = %+v", got)
	}

	all, err := discordcfg.All(app)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("All = %d guilds, want 1 (upsert didn't duplicate)", len(all))
	}

	if _, ok := discordcfg.Get(app, "unknown"); ok {
		t.Error("Get(unknown) should be (zero, false)")
	}
}
