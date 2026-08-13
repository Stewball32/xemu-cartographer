// Package discordcfg is cartographer's Discord config layer. It owns two things:
//
//   - The canonical guild→channel→hook ROUTING table (`discord_routes`) — see
//     bindings.go (GetBinding/SetBinding/DeleteBinding + the hook constants).
//   - The per-guild results-posting CONFIG built on top of it: which channel
//     game/series results post to (the `announcements` hook) and which series
//     categories a guild opted into (the `posted_categories` filter, stored in
//     `discord_guild_settings`).
//
// Post-migration (Discord bot spec): the old flat `discord_guilds` collection is
// retired. Results routing now reads the `announcements` route; the category
// filter reads `discord_guild_settings`. The category-filter logic below stays
// pure (unit-tested without a DB); Get / All / Upsert wrap the two collections.
// Consumed by the `/cartographer config` slash command (write) and the game-end
// post hook (read).
package discordcfg

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

const settingsCollection = "discord_guild_settings"

// GuildConfig is one guild's results-posting config, projected from the routing
// table (`announcements`/`tournament` hooks) + the category filter.
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

// Get returns a guild's config, or (zero, false) when unconfigured. Sources the
// channels from the routing table and the category filter from settings.
func Get(app core.App, guildID string) (GuildConfig, bool) {
	cfg := buildConfig(app, guildID)
	configured := cfg.ResultsChannel != "" || cfg.TournamentChannel != "" || len(cfg.PostedCategories) > 0
	return cfg, configured
}

// buildConfig projects the routing table + settings into a GuildConfig.
func buildConfig(app core.App, guildID string) GuildConfig {
	cfg := GuildConfig{GuildID: guildID}
	if ch, ok := GetBinding(app, guildID, HookAnnouncements); ok {
		cfg.ResultsChannel = ch
	}
	if ch, ok := GetBinding(app, guildID, HookTournament); ok {
		cfg.TournamentChannel = ch
	}
	if r, err := app.FindFirstRecordByFilter(settingsCollection,
		"guild_id = {:g}", dbx.Params{"g": guildID}); err == nil && r != nil {
		cfg.PostedCategories = r.GetStringSlice("posted_categories")
	}
	return cfg
}

// All returns every configured guild (the post-hook fan-out source): the union
// of guilds that have any route or any settings row.
func All(app core.App) ([]GuildConfig, error) {
	ids, err := configuredGuildIDs(app)
	if err != nil {
		return nil, err
	}
	out := make([]GuildConfig, 0, len(ids))
	for id := range ids {
		out = append(out, buildConfig(app, id))
	}
	return out, nil
}

// configuredGuildIDs collects the distinct guild ids across discord_routes +
// discord_guild_settings.
func configuredGuildIDs(app core.App) (map[string]struct{}, error) {
	ids := map[string]struct{}{}
	routes, err := app.FindAllRecords(RoutesCollection)
	if err != nil {
		return nil, err
	}
	for _, r := range routes {
		ids[r.GetString("guild_id")] = struct{}{}
	}
	settings, err := app.FindAllRecords(settingsCollection)
	if err != nil {
		return nil, err
	}
	for _, r := range settings {
		ids[r.GetString("guild_id")] = struct{}{}
	}
	delete(ids, "")
	return ids, nil
}

// Upsert writes (or updates) a guild's results-posting config — used by the
// `/cartographer config` slash command via app.Save (bypasses the collections'
// nil rules). The results channel folds into the `announcements` route; the
// category filter into discord_guild_settings.
func Upsert(app core.App, cfg GuildConfig) error {
	if cfg.ResultsChannel != "" {
		if err := SetBinding(app, cfg.GuildID, HookAnnouncements, cfg.ResultsChannel); err != nil {
			return err
		}
	}
	if cfg.TournamentChannel != "" {
		if err := SetBinding(app, cfg.GuildID, HookTournament, cfg.TournamentChannel); err != nil {
			return err
		}
	}
	return setGuildCategories(app, cfg.GuildID, cfg.PostedCategories)
}

// setGuildCategories upserts a guild's posted-category filter.
func setGuildCategories(app core.App, guildID string, categories []string) error {
	r, err := app.FindFirstRecordByFilter(settingsCollection,
		"guild_id = {:g}", dbx.Params{"g": guildID})
	if err != nil || r == nil {
		col, cerr := app.FindCollectionByNameOrId(settingsCollection)
		if cerr != nil {
			return cerr
		}
		r = core.NewRecord(col)
		r.Set("guild_id", guildID)
	}
	r.Set("posted_categories", categories)
	return app.Save(r)
}
