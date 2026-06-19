package commands

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/pocketbase/pocketbase/core"
)

// commandApp returns the PocketBase app from the injected Services, or nil when
// Services/App aren't wired (so handlers can degrade gracefully).
func commandApp() core.App {
	if s := services(); s != nil {
		return s.App
	}
	return nil
}

func replyEmbed(e *handler.CommandEvent, embed discord.Embed) error {
	return e.CreateMessage(discord.MessageCreate{Embeds: []discord.Embed{embed}})
}

func replyText(e *handler.CommandEvent, text string) error {
	return e.CreateMessage(discord.MessageCreate{Content: text})
}
