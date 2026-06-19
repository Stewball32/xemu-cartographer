package discord

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

// Notify abstracts sending messages to Discord channels.
type Notify interface {
	SendNotification(channelID snowflake.ID, content string) error
	// PostEmbed posts a single embed to a channel (M17 game/series/result
	// posting). Callers no-op when the bot isn't connected (svc.Discord nil).
	PostEmbed(channelID snowflake.ID, embed discord.Embed) error
}
