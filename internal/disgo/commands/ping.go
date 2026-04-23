package commands

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

// Registration: pairs the slash command definition with its handler.
func init() {
	register(Command{
		Create: discord.SlashCommandCreate{
			Name:        "ping",
			Description: "Replies with pong",
		},
		Handler: handlePing,
	})
}

// Handler: command logic goes here.
func handlePing(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	return e.CreateMessage(discord.MessageCreate{
		Content: "Pong!",
	})
}
