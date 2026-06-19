package hooks

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/routine"

	"github.com/Stewball32/xemu-cartographer/internal/discordcfg"
	"github.com/Stewball32/xemu-cartographer/internal/disgo/components/embeds"
)

func init() {
	register(registerGamesDiscordPost)
}

// registerGamesDiscordPost posts a finished-game result embed to each guild
// channel that opted into the game's series category (M17c). Fires on `games`
// insert — the path the M13 game-end chain writes through. Best-effort + async
// (routine.FireAndForget), and a complete no-op when the bot isn't connected,
// so it never affects persistence and is inert offline / in tests.
func registerGamesDiscordPost(app *pocketbase.PocketBase) {
	app.OnRecordAfterCreateSuccess("games").BindFunc(func(e *core.RecordEvent) error {
		postGameResult(e.App, e.Record)
		return e.Next()
	})
}

func postGameResult(app core.App, game *core.Record) {
	if svc == nil || svc.Discord == nil {
		return // bot not connected (offline / dev / tests)
	}
	embed, targets := gameResultTargets(app, game)
	if len(targets) == 0 {
		return
	}
	discordSvc := svc.Discord
	for _, ch := range targets {
		id, err := snowflake.Parse(ch)
		if err != nil {
			continue
		}
		routine.FireAndForget(func() {
			_ = discordSvc.PostEmbed(id, embed)
		})
	}
}

// gameResultTargets builds the result embed for a finished game + the guild
// channels that opted into its series category. Pure of Discord I/O so the
// resolution + category filter are unit-testable without a gateway.
func gameResultTargets(app core.App, game *core.Record) (discord.Embed, []string) {
	category := seriesCategory(app, game.GetString("series"))
	w := game.GetInt("winner_team")
	embed := embeds.GameResult(embeds.GameResultInput{
		Map:          game.GetString("map"),
		Gametype:     game.GetString("gametype"),
		VariantName:  game.GetString("variant_name"),
		ScoreSummary: game.GetString("score_summary"),
		Category:     category,
		WinnerTeam:   &w,
	})
	configs, err := discordcfg.All(app)
	if err != nil {
		return embed, nil
	}
	return embed, discordcfg.ResultsTargets(configs, category)
}

// seriesCategory resolves a series' category by id ("" when missing).
func seriesCategory(app core.App, seriesID string) string {
	if seriesID == "" {
		return ""
	}
	s, err := app.FindRecordById("series", seriesID)
	if err != nil || s == nil {
		return ""
	}
	return s.GetString("category")
}
