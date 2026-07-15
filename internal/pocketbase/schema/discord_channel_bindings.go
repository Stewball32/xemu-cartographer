package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerDiscordChannelBindingsCollection)
}

// registerDiscordChannelBindingsCollection creates the guild→channel→hook
// routing table: one row per (guild_id, hook) binding a Discord channel to a bot
// function. Normalized so new hooks are new ROWS, not schema changes — the
// `/setup` command writes these, and commands/events read them to know where to
// post.
//
//   - hook: the function slug ("category" is the structural parent /setup nests
//     under; the rest are postable targets — see internal/discordcfg hooks).
//   - channel_id: the Discord channel/category snowflake.
//
// nil rules — all access is server-side (slash-command write / event read),
// mirroring discord_guilds; needs no identity.go coordination.
func registerDiscordChannelBindingsCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "discord_channel_bindings") {
		return nil
	}

	collection := core.NewBaseCollection("discord_channel_bindings")
	collection.Fields.Add(
		&core.TextField{Name: "guild_id", Required: true, Min: 1, Max: 32},
		&core.SelectField{
			Name:      "hook",
			Required:  true,
			MaxSelect: 1,
			Values:    []string{"category", "container_status", "kiosk_links", "announcements", "bot_log"},
		},
		&core.TextField{Name: "channel_id", Required: true, Min: 1, Max: 32},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
	)

	// One channel per (guild, hook); a lookup index on guild_id for reads.
	collection.AddIndex("idx_discord_bindings_guild_hook_unique", true, "guild_id, hook", "")
	collection.AddIndex("idx_discord_bindings_guild", false, "guild_id", "")

	collection.ListRule = nil
	collection.ViewRule = nil
	collection.CreateRule = nil
	collection.UpdateRule = nil
	collection.DeleteRule = nil

	return app.Save(collection)
}
