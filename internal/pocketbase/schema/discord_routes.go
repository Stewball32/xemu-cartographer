package schema

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	// Registers AFTER discord_guild_settings (alphabetical: "discord_guild_settings"
	// < "discord_routes"), so the settings collection exists before the migration
	// folds discord_guilds.posted_categories into it.
	register(registerDiscordRoutesCollection)
}

// registerDiscordRoutesCollection creates the canonical guild→channel→hook
// routing table `discord_routes` (shared shape + name with the norcal-halo-site
// bot per the Discord bot spec) and MIGRATES the two older cart collections into
// it, then drops them:
//
//   - `discord_channel_bindings` (the /setup routing table, same rows but with a
//     `SelectField` enum `hook`) → copied verbatim into discord_routes.
//   - `discord_guilds` (flat M17a config) → its channel fields fold into hooks
//     (`results_channel` → `announcements`, `tournament_channel` → `tournament`),
//     and its `posted_categories` filter moves to `discord_guild_settings`.
//
// Canonical shape (per spec): `hook` is a FREE-TEXT key (max 64), not an enum —
// the two bots have different hook sets, so a shared enum would be brittle;
// validation lives a layer up (the `/config` UI only offers this app's known
// hooks — see internal/discordcfg). guild_id/channel_id are max-32 snowflakes;
// unique (guild_id, hook) + a (guild_id) lookup index for whole-guild reads.
//
// This is a versioned, explicit migration (create new → copy rows → drop old),
// NOT a silent rename: it runs once (guarded by existence checks), is idempotent
// on re-boot, and — because the old create-registrations were removed — never
// re-creates the retired collections. nil rules (server-side access only). The
// migration steps take core.App so they're unit-testable (see the test).
func registerDiscordRoutesCollection(app *pocketbase.PocketBase) error {
	if err := ensureRoutesCollection(app); err != nil {
		return err
	}
	// Fold the legacy /setup routing table (enum hook), then drop it.
	if err := migrateChannelBindings(app); err != nil {
		return err
	}
	// Fold the flat M17a config: channels → hooks, categories → guild_settings.
	return migrateDiscordGuilds(app)
}

// ensureRoutesCollection creates discord_routes if it doesn't exist yet.
func ensureRoutesCollection(app core.App) error {
	if _, err := app.FindCollectionByNameOrId("discord_routes"); err == nil {
		return nil
	}
	collection := core.NewBaseCollection("discord_routes")
	collection.Fields.Add(
		&core.TextField{Name: "guild_id", Required: true, Min: 1, Max: 32, Presentable: true},
		&core.TextField{Name: "hook", Required: true, Min: 1, Max: 64},
		&core.TextField{Name: "channel_id", Required: true, Min: 1, Max: 32},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
	)
	collection.AddIndex("idx_discord_routes_guild_hook_unique", true, "guild_id, hook", "")
	collection.AddIndex("idx_discord_routes_guild", false, "guild_id", "")
	collection.ListRule = nil
	collection.ViewRule = nil
	collection.CreateRule = nil
	collection.UpdateRule = nil
	collection.DeleteRule = nil
	return app.Save(collection)
}

// migrateChannelBindings copies every row of the retired
// `discord_channel_bindings` collection into discord_routes (upsert on
// (guild,hook)), then deletes the old collection. No-op when it's already gone.
func migrateChannelBindings(app core.App) error {
	old, err := app.FindCollectionByNameOrId("discord_channel_bindings")
	if err != nil || old == nil {
		return nil // already migrated / never existed
	}
	rows, err := app.FindAllRecords("discord_channel_bindings")
	if err != nil {
		return err
	}
	for _, r := range rows {
		if err := upsertRoute(app, r.GetString("guild_id"), r.GetString("hook"), r.GetString("channel_id")); err != nil {
			return err
		}
	}
	return app.Delete(old)
}

// migrateDiscordGuilds folds the retired flat `discord_guilds` collection:
// results_channel → `announcements` hook (only when a guild has no announcements
// route yet — an explicit /setup binding wins), tournament_channel →
// `tournament` hook, and posted_categories → discord_guild_settings. Then drops
// discord_guilds. No-op when already gone.
func migrateDiscordGuilds(app core.App) error {
	old, err := app.FindCollectionByNameOrId("discord_guilds")
	if err != nil || old == nil {
		return nil
	}
	rows, err := app.FindAllRecords("discord_guilds")
	if err != nil {
		return err
	}
	for _, r := range rows {
		guild := r.GetString("guild_id")
		if guild == "" {
			continue
		}
		if rc := r.GetString("results_channel"); rc != "" {
			if _, ok := findRoute(app, guild, "announcements"); !ok {
				if err := upsertRoute(app, guild, "announcements", rc); err != nil {
					return err
				}
			}
		}
		if tc := r.GetString("tournament_channel"); tc != "" {
			if err := upsertRoute(app, guild, "tournament", tc); err != nil {
				return err
			}
		}
		if cats := r.GetStringSlice("posted_categories"); len(cats) > 0 {
			if err := upsertGuildSettings(app, guild, cats); err != nil {
				return err
			}
		}
	}
	return app.Delete(old)
}

// upsertRoute writes (guild, hook) → channel into discord_routes (raw, so schema
// doesn't depend on internal/discordcfg).
func upsertRoute(app core.App, guildID, hook, channelID string) error {
	if guildID == "" || hook == "" || channelID == "" {
		return nil
	}
	r, ok := findRoute(app, guildID, hook)
	if !ok {
		col, err := app.FindCollectionByNameOrId("discord_routes")
		if err != nil {
			return err
		}
		r = core.NewRecord(col)
		r.Set("guild_id", guildID)
		r.Set("hook", hook)
	}
	r.Set("channel_id", channelID)
	return app.Save(r)
}

func findRoute(app core.App, guildID, hook string) (*core.Record, bool) {
	r, err := app.FindFirstRecordByFilter("discord_routes",
		"guild_id = {:g} && hook = {:h}", dbx.Params{"g": guildID, "h": hook})
	if err != nil || r == nil {
		return nil, false
	}
	return r, true
}

func upsertGuildSettings(app core.App, guildID string, categories []string) error {
	r, err := app.FindFirstRecordByFilter("discord_guild_settings",
		"guild_id = {:g}", dbx.Params{"g": guildID})
	if err != nil || r == nil {
		col, cerr := app.FindCollectionByNameOrId("discord_guild_settings")
		if cerr != nil {
			return cerr
		}
		r = core.NewRecord(col)
		r.Set("guild_id", guildID)
	}
	r.Set("posted_categories", categories)
	return app.Save(r)
}
