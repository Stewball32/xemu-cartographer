package commands

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/disgo/components/embeds"
	"github.com/Stewball32/xemu-cartographer/internal/stats"
)

// /recent user:<gamertag> (M17b) — the player's most recent games.
func init() {
	register(Command{
		Create: discord.SlashCommandCreate{
			Name:        "recent",
			Description: "Show a player's recent games",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionString{Name: "user", Description: "Gamertag", Required: true},
			},
		},
		Handler: handleRecent,
		Group:   "Stats",
	})
}

func handleRecent(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	app := commandApp()
	if app == nil {
		return replyText(e, "Stats are unavailable right now.")
	}
	gt := data.String("user")
	if gt == "" {
		return replyText(e, "Provide `user:<gamertag>`.")
	}
	return replyEmbed(e, recentEmbed(app, gt, 10))
}

func recentEmbed(app core.App, gamertag string, limit int) discord.Embed {
	lines, _ := stats.PlayerGamesForGamertag(app, gamertag)
	return embeds.RecentGames(gamertag, lines, limit)
}
