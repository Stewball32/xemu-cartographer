package actions

import (
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

// CreateVoiceChannel creates a new voice channel in the given guild.
func CreateVoiceChannel(client *bot.Client, guildID snowflake.ID, name string) (discord.GuildChannel, error) {
	return client.Rest.CreateGuildChannel(guildID, discord.GuildVoiceChannelCreate{
		Name: name,
	})
}
