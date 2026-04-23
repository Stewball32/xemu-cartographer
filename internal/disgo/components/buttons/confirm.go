package buttons

import "github.com/disgoorg/disgo/discord"

// Confirm returns a green "Yes" button with the given custom ID.
func Confirm(customID string) discord.ButtonComponent {
	return discord.NewSuccessButton("Yes", customID)
}
