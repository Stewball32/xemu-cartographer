package discord

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

// Voice abstracts Discord voice channel operations.
type Voice interface {
	CreateVoiceChannel(guildID snowflake.ID, name string) (discord.GuildChannel, error)
}
