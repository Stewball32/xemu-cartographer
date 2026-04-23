package rows

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/Stewball32/xemu-cartographer/internal/disgo/components/buttons"
)

// ConfirmRow returns an action row with a green "Yes" and red "No" button.
func ConfirmRow(yesID, noID string) discord.ActionRowComponent {
	return discord.NewActionRow(
		buttons.Confirm(yesID),
		buttons.Cancel(noID),
	)
}
