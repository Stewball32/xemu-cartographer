package schema

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// TestDiscordRoutesMigration verifies the versioned fold: legacy
// discord_channel_bindings + discord_guilds rows migrate into discord_routes
// (and discord_guild_settings), and the old collections are dropped.
func TestDiscordRoutesMigration(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)

	// Canonical targets (created at boot by their register funcs; here manually).
	if err := ensureRoutesCollection(app); err != nil {
		t.Fatalf("ensureRoutesCollection: %v", err)
	}
	settings := core.NewBaseCollection("discord_guild_settings")
	settings.Fields.Add(
		&core.TextField{Name: "guild_id", Required: true},
		&core.SelectField{Name: "posted_categories", MaxSelect: 4, Values: []string{"casual", "competitive", "tournament", "custom"}},
	)
	settings.AddIndex("idx_settings_g_mig", true, "guild_id", "")
	if err := app.Save(settings); err != nil {
		t.Fatalf("save settings: %v", err)
	}

	// Legacy /setup routing table + a couple rows.
	bindings := core.NewBaseCollection("discord_channel_bindings")
	bindings.Fields.Add(
		&core.TextField{Name: "guild_id", Required: true},
		&core.SelectField{Name: "hook", Required: true, MaxSelect: 1,
			Values: []string{"category", "container_status", "kiosk_links", "announcements", "bot_log"}},
		&core.TextField{Name: "channel_id", Required: true},
	)
	bindings.AddIndex("idx_bind_gh_mig", true, "guild_id, hook", "")
	if err := app.Save(bindings); err != nil {
		t.Fatalf("save bindings: %v", err)
	}
	saveMig(t, app, "discord_channel_bindings", map[string]any{"guild_id": "g1", "hook": "announcements", "channel_id": "aaa"})
	saveMig(t, app, "discord_channel_bindings", map[string]any{"guild_id": "g1", "hook": "bot_log", "channel_id": "bbb"})

	// Legacy flat config + rows: g1 already has an announcements binding (must not
	// be clobbered); g2 has results_channel that should fold to announcements.
	guilds := core.NewBaseCollection("discord_guilds")
	guilds.Fields.Add(
		&core.TextField{Name: "guild_id", Required: true},
		&core.TextField{Name: "results_channel"},
		&core.TextField{Name: "tournament_channel"},
		&core.SelectField{Name: "posted_categories", MaxSelect: 4, Values: []string{"casual", "competitive", "tournament", "custom"}},
	)
	guilds.AddIndex("idx_guilds_g_mig", true, "guild_id", "")
	if err := app.Save(guilds); err != nil {
		t.Fatalf("save guilds: %v", err)
	}
	saveMig(t, app, "discord_guilds", map[string]any{"guild_id": "g1", "results_channel": "should-not-win", "posted_categories": []string{"competitive"}})
	saveMig(t, app, "discord_guilds", map[string]any{"guild_id": "g2", "results_channel": "ccc", "tournament_channel": "ddd", "posted_categories": []string{"casual"}})

	// Run the migration.
	if err := migrateChannelBindings(app); err != nil {
		t.Fatalf("migrateChannelBindings: %v", err)
	}
	if err := migrateDiscordGuilds(app); err != nil {
		t.Fatalf("migrateDiscordGuilds: %v", err)
	}

	// Old collections dropped.
	if _, err := app.FindCollectionByNameOrId("discord_channel_bindings"); err == nil {
		t.Error("discord_channel_bindings should be dropped")
	}
	if _, err := app.FindCollectionByNameOrId("discord_guilds"); err == nil {
		t.Error("discord_guilds should be dropped")
	}

	// g1 bindings copied verbatim; the explicit announcements route wins over the
	// legacy results_channel (not clobbered).
	assertRoute(t, app, "g1", "announcements", "aaa")
	assertRoute(t, app, "g1", "bot_log", "bbb")
	// g2 results_channel folded to announcements; tournament_channel to tournament.
	assertRoute(t, app, "g2", "announcements", "ccc")
	assertRoute(t, app, "g2", "tournament", "ddd")

	// posted_categories moved to settings for both guilds.
	assertCategories(t, app, "g1", "competitive")
	assertCategories(t, app, "g2", "casual")
}

func saveMig(t *testing.T, app core.App, col string, set map[string]any) {
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
}

func assertRoute(t *testing.T, app core.App, guild, hook, want string) {
	t.Helper()
	r, ok := findRoute(app, guild, hook)
	if !ok {
		t.Errorf("route (%s,%s) missing, want channel %q", guild, hook, want)
		return
	}
	if got := r.GetString("channel_id"); got != want {
		t.Errorf("route (%s,%s) channel = %q, want %q", guild, hook, got, want)
	}
}

func assertCategories(t *testing.T, app core.App, guild, wantCat string) {
	t.Helper()
	r, err := app.FindFirstRecordByFilter("discord_guild_settings", "guild_id = {:g}", map[string]any{"g": guild})
	if err != nil || r == nil {
		t.Errorf("settings for %s missing", guild)
		return
	}
	cats := r.GetStringSlice("posted_categories")
	if len(cats) != 1 || cats[0] != wantCat {
		t.Errorf("settings %s categories = %v, want [%s]", guild, cats, wantCat)
	}
}
