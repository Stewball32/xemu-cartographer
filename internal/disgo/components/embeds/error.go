package embeds

import "github.com/disgoorg/disgo/discord"

// Error returns a red embed with the given title and description.
func Error(title, description string) discord.Embed {
	return discord.Embed{
		Title:       title,
		Description: description,
		Color:       0xED4245,
	}
}
