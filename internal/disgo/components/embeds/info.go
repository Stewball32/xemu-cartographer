package embeds

import "github.com/disgoorg/disgo/discord"

// Info returns a blue embed with the given title and description.
func Info(title, description string) discord.Embed {
	return discord.Embed{
		Title:       title,
		Description: description,
		Color:       0x5865F2,
	}
}
