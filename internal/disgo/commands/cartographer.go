package commands

import (
	"fmt"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/omit"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/discordcfg"
	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
)

// /cartographer — the ops + config command group (M17a config + the old-M8 ops
// commands folded in). `config` writes the per-guild posting config; `status`
// shows who's playing now. Config write + the status embed are testable; the
// option/interaction plumbing is verified live.
func init() {
	register(Command{
		Create: discord.SlashCommandCreate{
			Name:        "cartographer",
			Description: "Cartographer config + ops",
			// Gate the ops/config group to Manage-Server users by default
			// (guild admins can adjust per-command perms in Discord settings).
			// /stats + /recent stay public.
			DefaultMemberPermissions: omit.NewPtr(discord.PermissionManageGuild),
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{
					Name:        "config",
					Description: "Set this server's result-posting config",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionString{Name: "results_channel", Description: "Channel ID for game results"},
						discord.ApplicationCommandOptionString{Name: "categories", Description: "Comma-separated: casual,competitive,tournament,custom"},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "status",
					Description: "Who's playing right now",
				},
			},
		},
		Handler: handleCartographer,
	})
}

func handleCartographer(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	sub := ""
	if data.SubCommandName != nil {
		sub = *data.SubCommandName
	}
	switch sub {
	case "config":
		return handleConfig(data, e)
	case "status":
		return handleStatus(e)
	default:
		return replyText(e, "Unknown subcommand.")
	}
}

func handleConfig(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	app := commandApp()
	if app == nil {
		return replyText(e, "Config is unavailable right now.")
	}
	gid := e.GuildID()
	if gid == nil {
		return replyText(e, "Use `/cartographer config` inside a server.")
	}
	results := data.String("results_channel")
	cats := data.String("categories")
	if err := applyConfig(app, gid.String(), results, cats); err != nil {
		return replyText(e, "Failed to save config.")
	}
	return replyText(e, fmt.Sprintf("Saved. Posting %s results to <#%s>.", cats, results))
}

func handleStatus(e *handler.CommandEvent) error {
	s := services()
	if s == nil || s.Scraper == nil {
		return replyText(e, "The scraper isn't running.")
	}
	return replyEmbed(e, statusEmbed(s.Scraper.Membership()))
}

// applyConfig upserts a guild's posting config from the parsed slash-command
// options. Testable without an interaction.
func applyConfig(app core.App, guildID, resultsChannel, categoriesCSV string) error {
	return discordcfg.Upsert(app, discordcfg.GuildConfig{
		GuildID:          guildID,
		ResultsChannel:   resultsChannel,
		PostedCategories: parseCategories(categoriesCSV),
	})
}

var validCategories = map[string]bool{"casual": true, "competitive": true, "tournament": true, "custom": true}

// parseCategories parses a comma-separated category list, lower-casing +
// dropping anything not a known series category.
func parseCategories(csv string) []string {
	var out []string
	for _, p := range strings.Split(csv, ",") {
		p = strings.TrimSpace(strings.ToLower(p))
		if validCategories[p] {
			out = append(out, p)
		}
	}
	return out
}

// statusEmbed renders the live "who's playing" view from the scraper roster
// membership. Pure.
func statusEmbed(view []scraperiface.ContainerMembership) discord.Embed {
	e := discord.Embed{Title: "Who's playing", Color: 0x5865f2}
	if len(view) == 0 {
		e.Description = "No active containers."
		return e
	}
	for _, m := range view {
		e.Fields = append(e.Fields, discord.EmbedField{
			Name:  m.Container,
			Value: fmt.Sprintf("%d on roster", len(m.Identities)),
		})
	}
	return e
}
