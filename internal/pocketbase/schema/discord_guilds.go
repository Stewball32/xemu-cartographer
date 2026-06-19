package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerDiscordGuildsCollection)
}

// registerDiscordGuildsCollection creates the M17a per-guild config: where (if
// anywhere) a Discord guild wants game/series/tournament results posted, and
// which series categories to post. One row per guild_id.
//
// `posted_categories` is an opt-in allowlist (empty = post nothing), so a guild
// never gets spammed until an operator configures it via `/cartographer config`.
// No relations + nil rules (config is written server-side by the slash-command
// handler via app.Save, read by the post hook via FindRecords) — so it needs no
// identity.go coordination and registers via its own init().
func registerDiscordGuildsCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "discord_guilds") {
		return nil
	}

	collection := core.NewBaseCollection("discord_guilds")
	collection.Fields.Add(
		&core.TextField{Name: "guild_id", Required: true, Min: 1, Max: 32, Presentable: true},
		&core.TextField{Name: "results_channel", Max: 32},
		&core.TextField{Name: "tournament_channel", Max: 32},
		&core.SelectField{
			Name:      "posted_categories",
			MaxSelect: 4,
			Values:    []string{"casual", "competitive", "tournament", "custom"},
		},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
	)

	collection.AddIndex("idx_discord_guilds_guild_id_unique", true, "guild_id", "")

	// nil rules — all access is server-side (slash-command write / post-hook read).
	collection.ListRule = nil
	collection.ViewRule = nil
	collection.CreateRule = nil
	collection.UpdateRule = nil
	collection.DeleteRule = nil

	return app.Save(collection)
}
