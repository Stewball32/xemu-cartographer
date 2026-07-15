package discordcfg

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// Channels is a guild's channel→hook routing — the bindings `/setup` provisions
// and persists on the EXISTING discord_guilds row so commands/events know where
// to post. All values are Discord channel/category snowflake IDs; empty means
// "not provisioned".
type Channels struct {
	Category        string // the "Cartographer" category the channels live under
	ContainerStatus string // live "who's playing"
	KioskLinks      string // per-player play/kiosk links
	Announcements   string // results / tournament posts
	BotLog          string // bot ops log
}

func channelsFromRecord(r *core.Record) Channels {
	return Channels{
		Category:        r.GetString("setup_category"),
		ContainerStatus: r.GetString("container_status_channel"),
		KioskLinks:      r.GetString("kiosk_links_channel"),
		Announcements:   r.GetString("announcements_channel"),
		BotLog:          r.GetString("bot_log_channel"),
	}
}

// GetChannels returns a guild's provisioned channel bindings, or (zero, false)
// when the guild is unconfigured.
func GetChannels(app core.App, guildID string) (Channels, bool) {
	r, err := app.FindFirstRecordByFilter("discord_guilds", "guild_id = {:g}", dbx.Params{"g": guildID})
	if err != nil || r == nil {
		return Channels{}, false
	}
	return channelsFromRecord(r), true
}

// SetChannels persists a guild's channel bindings, creating the guild row if it
// doesn't exist. Read-modify-write of ONLY the binding fields, so it never
// clobbers the posting config (results_channel / posted_categories) that
// `/cartographer config` writes to the same row.
func SetChannels(app core.App, guildID string, ch Channels) error {
	r, err := app.FindFirstRecordByFilter("discord_guilds", "guild_id = {:g}", dbx.Params{"g": guildID})
	if err != nil || r == nil {
		col, cerr := app.FindCollectionByNameOrId("discord_guilds")
		if cerr != nil {
			return cerr
		}
		r = core.NewRecord(col)
		r.Set("guild_id", guildID)
	}
	r.Set("setup_category", ch.Category)
	r.Set("container_status_channel", ch.ContainerStatus)
	r.Set("kiosk_links_channel", ch.KioskLinks)
	r.Set("announcements_channel", ch.Announcements)
	r.Set("bot_log_channel", ch.BotLog)
	return app.Save(r)
}
