package commands

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/disgo/components/embeds"
	"github.com/Stewball32/xemu-cartographer/internal/rating"
)

const leaderboardTopN = 10

// /leaderboard type:<gametype> and /rank user:<gamertag> type:<gametype> (M18c/d)
// over the per-game-type `ratings` store the M18b game-end chain maintains.
func init() {
	register(Command{
		Create: discord.SlashCommandCreate{
			Name:        "leaderboard",
			Description: "Top players by rating for a game type",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionString{Name: "type", Description: "Game type (e.g. slayer)", Required: true},
			},
		},
		Handler: handleLeaderboard,
		Group:   "Stats",
	})
	register(Command{
		Create: discord.SlashCommandCreate{
			Name:        "rank",
			Description: "A player's rating + rank in a game type",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionString{Name: "user", Description: "Gamertag", Required: true},
				discord.ApplicationCommandOptionString{Name: "type", Description: "Game type (e.g. slayer)", Required: true},
			},
		},
		Handler: handleRank,
		Group:   "Stats",
	})
}

func handleLeaderboard(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	app := commandApp()
	if app == nil {
		return replyText(e, "Leaderboards are unavailable right now.")
	}
	gt := data.String("type")
	entries := rating.TopN(leaderboardEntries(app, gt), leaderboardTopN)
	return replyEmbed(e, embeds.Leaderboard(gt, entries))
}

func handleRank(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	app := commandApp()
	if app == nil {
		return replyText(e, "Ranks are unavailable right now.")
	}
	gt := data.String("type")
	gamertag := data.String("user")
	entries := leaderboardEntries(app, gt)
	rank := rating.RankOf(entries, gamertag)
	var r float64
	for _, en := range entries {
		if en.Gamertag == gamertag {
			r = en.Rating
		}
	}
	return replyEmbed(e, embeds.Rank(gamertag, gt, rank, r))
}

// leaderboardEntries reads the ratings store for a game type into rating.Entry
// values (M18c). Testable against a test app.
func leaderboardEntries(app core.App, gametype string) []rating.Entry {
	rows, err := app.FindRecordsByFilter("ratings", "gametype = {:gt}", "-rating", 0, 0, dbx.Params{"gt": gametype})
	if err != nil {
		return nil
	}
	out := make([]rating.Entry, 0, len(rows))
	for _, r := range rows {
		out = append(out, rating.Entry{
			Gamertag: r.GetString("gamertag"),
			Rating:   r.GetFloat("rating"),
			Games:    r.GetInt("games"),
		})
	}
	return out
}
