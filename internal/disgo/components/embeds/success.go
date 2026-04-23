package embeds

import "github.com/disgoorg/disgo/discord"

// Success returns a green embed with the given title and description.
func Success(title, description string) discord.Embed {
	return discord.Embed{
		Title:       title,
		Description: description,
		Color:       0x57F287,
	}
}
