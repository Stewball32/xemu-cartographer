package commands

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/disgo/components/embeds"
	"github.com/Stewball32/xemu-cartographer/internal/stats"
)

// /stats user:<gamertag> | team:<slug> (M17b). Resolution + embed building is
// in the testable userStatsEmbed / teamStatsEmbed helpers; the handler is thin
// option-plumbing (verified live once the gateway is connected).
func init() {
	register(Command{
		Create: discord.SlashCommandCreate{
			Name:        "stats",
			Description: "Show player or team stats",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionString{Name: "user", Description: "Gamertag"},
				discord.ApplicationCommandOptionString{Name: "team", Description: "Team slug"},
			},
		},
		Handler: handleStats,
		Group:   "Stats",
	})
}

func handleStats(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	app := commandApp()
	if app == nil {
		return replyText(e, "Stats are unavailable right now.")
	}
	if gt, ok := data.OptString("user"); ok && gt != "" {
		return replyEmbed(e, userStatsEmbed(app, gt))
	}
	if slug, ok := data.OptString("team"); ok && slug != "" {
		return replyEmbed(e, teamStatsEmbed(app, slug))
	}
	return replyText(e, "Provide `user:<gamertag>` or `team:<slug>`.")
}

// userStatsEmbed resolves a gamertag's stats (M15) into a stats embed.
func userStatsEmbed(app core.App, gamertag string) discord.Embed {
	lines, err := stats.PlayerGamesForGamertag(app, gamertag)
	if err != nil {
		return embeds.UserStats(gamertag, stats.Totals{}, nil)
	}
	return embeds.UserStats(gamertag, stats.Roll(lines), stats.RollByGametype(lines))
}

// teamStatsEmbed rolls a team's active roster's gamertags into one stats embed.
func teamStatsEmbed(app core.App, slug string) discord.Embed {
	tags := teamGamertags(app, slug)
	lines, _ := stats.PlayerGamesForGamertags(app, tags)
	return embeds.UserStats("Team "+slug, stats.Roll(lines), stats.RollByGametype(lines))
}

// teamGamertags resolves a team's active roster gamertag strings (matching the
// game_players.gamertag the stats query keys on).
func teamGamertags(app core.App, slug string) []string {
	team, err := app.FindFirstRecordByFilter("teams", "slug = {:s}", dbx.Params{"s": slug})
	if err != nil || team == nil {
		return nil
	}
	rosters, err := app.FindRecordsByFilter("rosters", "team = {:t} && left_at = null", "", 0, 0, dbx.Params{"t": team.Id})
	if err != nil {
		return nil
	}
	_ = app.ExpandRecords(rosters, []string{"gamertag"}, nil)
	var tags []string
	for _, r := range rosters {
		if gt := r.ExpandedOne("gamertag"); gt != nil {
			if tag := gt.GetString("tag"); tag != "" {
				tags = append(tags, tag)
			}
		}
	}
	return tags
}
