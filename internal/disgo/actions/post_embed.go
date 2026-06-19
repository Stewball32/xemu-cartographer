package actions

import (
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

// PostEmbed posts a single embed to the given channel (M17 event posting).
func PostEmbed(client *bot.Client, channelID snowflake.ID, embed discord.Embed) error {
	_, err := client.Rest.CreateMessage(channelID, discord.MessageCreate{
		Embeds: []discord.Embed{embed},
	})
	return err
}
