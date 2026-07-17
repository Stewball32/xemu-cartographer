package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerDiscordGuildSettingsCollection)
}

// registerDiscordGuildSettingsCollection creates `discord_guild_settings` — the
// small per-guild settings that are NOT channel routing. Today it holds only the
// results-posting category filter (`posted_categories`), migrated out of the
// retired flat `discord_guilds` collection when the routing side moved to the
// canonical `discord_routes` table (see discord_routes.go).
//
// Kept separate from `discord_routes` on purpose: `posted_categories` is an
// opt-in FILTER (which series categories a guild wants results for), not a
// guild→channel→hook binding. Empty = post nothing, so a guild is never spammed
// until an operator opts in via `/cartographer config`.
//
// One row per guild_id, nil rules (written server-side by the slash-command
// handler via app.Save, read by the results post hook), own init() — no
// identity.go coordination (no relations, no user_roles rule subquery).
func registerDiscordGuildSettingsCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "discord_guild_settings") {
		return nil
	}

	collection := core.NewBaseCollection("discord_guild_settings")
	collection.Fields.Add(
		&core.TextField{Name: "guild_id", Required: true, Min: 1, Max: 32, Presentable: true},
		&core.SelectField{
			Name:      "posted_categories",
			MaxSelect: 4,
			Values:    []string{"casual", "competitive", "tournament", "custom"},
		},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
	)

	collection.AddIndex("idx_discord_guild_settings_guild_id_unique", true, "guild_id", "")

	collection.ListRule = nil
	collection.ViewRule = nil
	collection.CreateRule = nil
	collection.UpdateRule = nil
	collection.DeleteRule = nil

	return app.Save(collection)
}
