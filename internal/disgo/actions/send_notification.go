package actions

import (
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

// SendNotification sends a text message to the given channel.
func SendNotification(client *bot.Client, channelID snowflake.ID, content string) error {
	_, err := client.Rest.CreateMessage(channelID, discord.MessageCreate{
		Content: content,
	})
	return err
}
