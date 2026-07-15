package discordcfg

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// Channel-binding hooks — the bot function each provisioned channel serves.
// Stored one row per (guild, hook) in the discord_channel_bindings collection,
// so new hooks are new rows (data), not schema changes. `/setup` writes them;
// commands/events read them to know where to post.
const (
	HookCategory        = "category"         // structural parent /setup nests channels under
	HookContainerStatus = "container_status" // live "who's playing"
	HookKioskLinks      = "kiosk_links"      // per-player play/kiosk links
	HookAnnouncements   = "announcements"    // results / tournament posts
	HookBotLog          = "bot_log"          // bot ops log
)

// PostHooks are the hooks the bot posts to (excludes the structural category).
var PostHooks = []string{HookContainerStatus, HookKioskLinks, HookAnnouncements, HookBotLog}

const bindingsCollection = "discord_channel_bindings"

// GetBinding returns the channel ID bound to (guild, hook), or ("", false).
func GetBinding(app core.App, guildID, hook string) (string, bool) {
	r, err := app.FindFirstRecordByFilter(bindingsCollection,
		"guild_id = {:g} && hook = {:h}", dbx.Params{"g": guildID, "h": hook})
	if err != nil || r == nil {
		return "", false
	}
	return r.GetString("channel_id"), true
}

// GetBindings returns a guild's full hook→channel map (empty when unconfigured).
func GetBindings(app core.App, guildID string) (map[string]string, error) {
	rows, err := app.FindRecordsByFilter(bindingsCollection,
		"guild_id = {:g}", "", 0, 0, dbx.Params{"g": guildID})
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.GetString("hook")] = r.GetString("channel_id")
	}
	return out, nil
}

// SetBinding upserts the channel bound to (guild, hook).
func SetBinding(app core.App, guildID, hook, channelID string) error {
	r, err := app.FindFirstRecordByFilter(bindingsCollection,
		"guild_id = {:g} && hook = {:h}", dbx.Params{"g": guildID, "h": hook})
	if err != nil || r == nil {
		col, cerr := app.FindCollectionByNameOrId(bindingsCollection)
		if cerr != nil {
			return cerr
		}
		r = core.NewRecord(col)
		r.Set("guild_id", guildID)
		r.Set("hook", hook)
	}
	r.Set("channel_id", channelID)
	return app.Save(r)
}
