package buttons

import "github.com/disgoorg/disgo/discord"

// Cancel returns a red "No" button with the given custom ID.
func Cancel(customID string) discord.ButtonComponent {
	return discord.NewDangerButton("No", customID)
}
