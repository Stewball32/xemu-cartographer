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
// setupChannelFields are the per-guild channel→hook bindings the `/setup`
// slash-command provisions and persists (a category + one channel per bot
// function), so setup is idempotent and the bindings survive restarts. Added to
// the EXISTING discord_guilds collection (M17a) rather than a new one — it's the
// established home for per-guild Discord config. Reused by internal/discordcfg.
func setupChannelFields() []core.Field {
	return []core.Field{
		&core.TextField{Name: "setup_category", Max: 32},           // category channel ID
		&core.TextField{Name: "container_status_channel", Max: 32}, // live "who's playing"
		&core.TextField{Name: "kiosk_links_channel", Max: 32},      // per-player play/kiosk links
		&core.TextField{Name: "announcements_channel", Max: 32},    // results / tournament posts
		&core.TextField{Name: "bot_log_channel", Max: 32},          // bot ops log
	}
}

func registerDiscordGuildsCollection(app *pocketbase.PocketBase) error {
	// Upgrade path for an existing collection: add any missing setup-channel
	// fields (idempotent; mirrors game_events.go), keeping prod DBs current
	// without a full migration framework.
	if existing, err := app.FindCollectionByNameOrId("discord_guilds"); err == nil {
		changed := false
		for _, f := range setupChannelFields() {
			if existing.Fields.GetByName(f.GetName()) == nil {
				existing.Fields.Add(f)
				changed = true
			}
		}
		if !changed {
			return nil
		}
		return app.Save(existing)
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
	collection.Fields.Add(setupChannelFields()...)

	collection.AddIndex("idx_discord_guilds_guild_id_unique", true, "guild_id", "")

	// nil rules — all access is server-side (slash-command write / post-hook read).
	collection.ListRule = nil
	collection.ViewRule = nil
	collection.CreateRule = nil
	collection.UpdateRule = nil
	collection.DeleteRule = nil

	return app.Save(collection)
}
