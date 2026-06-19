// Package discordcfg is the M17a per-guild Discord config layer: where each
// guild wants results/tournament posts and which series categories it opted
// into. The category-filter logic is pure (unit-tested without a DB); Get /
// Upsert / All wrap the discord_guilds collection. Consumed by the
// /cartographer config slash command (write) and the game-end post hook (read).
package discordcfg

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// GuildConfig is one guild's posting config.
type GuildConfig struct {
	GuildID           string
	ResultsChannel    string
	TournamentChannel string
	// PostedCategories is the opt-in allowlist of series categories
	// (casual/competitive/tournament/custom). Empty = post nothing.
	PostedCategories []string
}

func (g GuildConfig) postsCategory(cat string) bool {
	for _, c := range g.PostedCategories {
		if c == cat {
			return true
		}
	}
	return false
}

// ResultsTarget returns the channel a game/series result of `category` should be
// posted to for this guild, or "" when it shouldn't (category not opted in, or
// no results channel configured). This is the per-guild spam filter.
func (g GuildConfig) ResultsTarget(category string) string {
	if g.ResultsChannel == "" || !g.postsCategory(category) {
		return ""
	}
	return g.ResultsChannel
}

// ResultsTargets returns every channel that should receive a result post for
// `category` across the given configs — the fan-out the post hook iterates.
func ResultsTargets(configs []GuildConfig, category string) []string {
	var out []string
	for _, g := range configs {
		if ch := g.ResultsTarget(category); ch != "" {
			out = append(out, ch)
		}
	}
	return out
}

func fromRecord(r *core.Record) GuildConfig {
	return GuildConfig{
		GuildID:           r.GetString("guild_id"),
		ResultsChannel:    r.GetString("results_channel"),
		TournamentChannel: r.GetString("tournament_channel"),
		PostedCategories:  r.GetStringSlice("posted_categories"),
	}
}

// Get returns a guild's config, or (zero, false) when unconfigured.
func Get(app core.App, guildID string) (GuildConfig, bool) {
	r, err := app.FindFirstRecordByFilter("discord_guilds", "guild_id = {:g}", dbx.Params{"g": guildID})
	if err != nil || r == nil {
		return GuildConfig{}, false
	}
	return fromRecord(r), true
}

// All returns every configured guild (the post-hook fan-out source).
func All(app core.App) ([]GuildConfig, error) {
	rows, err := app.FindAllRecords("discord_guilds")
	if err != nil {
		return nil, err
	}
	out := make([]GuildConfig, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromRecord(r))
	}
	return out, nil
}

// Upsert writes (or updates) a guild's config — used by the /cartographer config
// slash command via app.Save (which bypasses the collection's nil rules).
func Upsert(app core.App, cfg GuildConfig) error {
	r, err := app.FindFirstRecordByFilter("discord_guilds", "guild_id = {:g}", dbx.Params{"g": cfg.GuildID})
	if err != nil || r == nil {
		col, cerr := app.FindCollectionByNameOrId("discord_guilds")
		if cerr != nil {
			return cerr
		}
		r = core.NewRecord(col)
		r.Set("guild_id", cfg.GuildID)
	}
	r.Set("results_channel", cfg.ResultsChannel)
	r.Set("tournament_channel", cfg.TournamentChannel)
	r.Set("posted_categories", cfg.PostedCategories)
	return app.Save(r)
}
