package commands

import (
	"fmt"
	"sort"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

// DefaultGroup is where commands with no Group land in /help.
const DefaultGroup = "Commands"

// helpBrandColor / helpMutedColor mirror the bot's existing embed palette
// (blurple brand, red for "not found").
const (
	helpBrandColor = 0x5865f2
	helpMutedColor = 0xed4245
)

// botDocsURL is linked from the /help overview for things a person may need to
// know that no command owns yet. When a command comes to own a concept, hang the
// write-up off that command's /help entry instead of linking out.
const botDocsURL = "https://github.com/Stewball32/xemu-cartographer#readme"

// /help — a registry-generated, permission-aware command reference. Everything
// is derived from the command registry (no hand-maintained lists, so it can't
// drift). See the canonical pattern in the shared bot spec.
//
//	/help                → overview: your usable commands, grouped for readability
//	/help command:<name> → the full entry for one command
//
// Only the commands the invoking member can actually use are listed and
// autocompleted, mirroring how Discord hides commands they lack permission for.
func init() {
	register(Command{
		Create: discord.SlashCommandCreate{
			Name:        "help",
			Description: "List the bot's commands, or explain one",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionString{
					Name:         "command",
					Description:  "A command to explain in full",
					Autocomplete: true,
				},
			},
		},
		Handler: handleHelp,
		Group:   "General",
	})
	registerAutocomplete(AutocompleteRoute{Pattern: "/help", Handler: helpAutocomplete})
}

func handleHelp(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	perms := resolvedPerms(e.Member())
	if name, ok := data.OptString("command"); ok && strings.TrimSpace(name) != "" {
		return replyEmbedEphemeral(e, helpDetail(strings.ToLower(strings.TrimSpace(name)), perms))
	}
	return replyEmbedEphemeral(e, helpOverview(perms))
}

// helpAutocomplete suggests only the commands the member can use, prefix-matched.
func helpAutocomplete(e *handler.AutocompleteEvent) error {
	perms := resolvedPerms(e.Member())
	typed := strings.ToLower(strings.TrimSpace(e.Data.String("command")))

	names := make([]string, 0, len(All()))
	for _, c := range All() {
		if c.Create.Name == "help" || !memberCanUse(perms, c) {
			continue
		}
		if typed == "" || strings.Contains(c.Create.Name, typed) {
			names = append(names, c.Create.Name)
		}
	}
	sort.Strings(names)

	choices := make([]discord.AutocompleteChoice, 0, len(names))
	for _, n := range names {
		if len(choices) >= 25 { // Discord's cap
			break
		}
		choices = append(choices, discord.AutocompleteChoiceString{Name: "/" + n, Value: n})
	}
	return e.AutocompleteResult(choices)
}

// helpOverview builds the grouped list of usable commands + the docs link.
func helpOverview(perms discord.Permissions) discord.Embed {
	byGroup := map[string][]Command{}
	for _, c := range All() {
		if !memberCanUse(perms, c) {
			continue
		}
		g := c.Group
		if g == "" {
			g = DefaultGroup
		}
		byGroup[g] = append(byGroup[g], c)
	}

	groups := make([]string, 0, len(byGroup))
	for g := range byGroup {
		groups = append(groups, g)
	}
	sort.Strings(groups)

	var b strings.Builder
	for _, g := range groups {
		cmds := byGroup[g]
		sort.Slice(cmds, func(i, j int) bool { return cmds[i].Create.Name < cmds[j].Create.Name })
		fmt.Fprintf(&b, "**%s**\n", g)
		for _, c := range cmds {
			fmt.Fprintf(&b, "`/%s` — %s\n", c.Create.Name, c.Create.Description)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "Use `/help command:<name>` for a command's full details.\n")
	fmt.Fprintf(&b, "📖 About the bot + setup: [docs](%s)", botDocsURL)

	return discord.Embed{Title: "Xemu Cartographer — commands", Description: b.String(), Color: helpBrandColor}
}

// helpDetail builds the full entry for one command (if the member can use it).
func helpDetail(name string, perms discord.Permissions) discord.Embed {
	var cmd *Command
	for i := range registry {
		if registry[i].Create.Name == name {
			cmd = &registry[i]
			break
		}
	}
	if cmd == nil || !memberCanUse(perms, *cmd) {
		return discord.Embed{
			Title:       "Unknown command",
			Description: fmt.Sprintf("No command `/%s` you can use. Try `/help` for the list.", name),
			Color:       helpMutedColor,
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n**Access:** %s\n", cmd.Create.Description, permLabel(*cmd))

	var subs, opts []discord.ApplicationCommandOption
	for _, o := range cmd.Create.Options {
		if o.Type() == discord.ApplicationCommandOptionTypeSubCommand ||
			o.Type() == discord.ApplicationCommandOptionTypeSubCommandGroup {
			subs = append(subs, o)
		} else {
			opts = append(opts, o)
		}
	}

	if len(subs) > 0 {
		b.WriteString("\n**Subcommands**\n")
		for _, s := range subs {
			n, d, _ := optInfo(s)
			fmt.Fprintf(&b, "`/%s %s` — %s\n", cmd.Create.Name, n, d)
			for _, so := range subOptions(s) {
				on, od, req := optInfo(so)
				fmt.Fprintf(&b, "   • `%s`%s — %s\n", on, requiredTag(req), od)
			}
		}
	}
	if len(opts) > 0 {
		b.WriteString("\n**Options**\n")
		for _, o := range opts {
			on, od, req := optInfo(o)
			fmt.Fprintf(&b, "• `%s`%s — %s\n", on, requiredTag(req), od)
		}
	}

	return discord.Embed{Title: "/" + cmd.Create.Name, Description: b.String(), Color: helpBrandColor}
}

// --- permission mirroring (best-effort: reflects default_member_permissions;
// per-guild admin overrides in Server Settings → Integrations aren't fetched) ---

func resolvedPerms(m *discord.ResolvedMember) discord.Permissions {
	if m == nil {
		return 0
	}
	return m.Permissions
}

func memberCanUse(perms discord.Permissions, cmd Command) bool {
	req := cmd.Create.DefaultMemberPermissions.Or(nil)
	if req == nil || *req == 0 {
		return true // open to everyone
	}
	if perms.Has(discord.PermissionAdministrator) {
		return true
	}
	return perms.Has(*req)
}

func permLabel(cmd Command) string {
	req := cmd.Create.DefaultMemberPermissions.Or(nil)
	if req == nil || *req == 0 {
		return "Everyone"
	}
	switch {
	case req.Has(discord.PermissionAdministrator):
		return "Administrator"
	case req.Has(discord.PermissionManageGuild):
		return "Manage Server"
	default:
		return "Elevated permissions"
	}
}

func requiredTag(required bool) string {
	if required {
		return " (required)"
	}
	return ""
}

// optInfo returns an option's name/description/required across the option types
// these bots use (subcommands + the common leaf types). Unknown types fall back
// to the interface's name.
func optInfo(opt discord.ApplicationCommandOption) (name, desc string, required bool) {
	switch o := opt.(type) {
	case discord.ApplicationCommandOptionSubCommand:
		return o.Name, o.Description, false
	case discord.ApplicationCommandOptionSubCommandGroup:
		return o.Name, o.Description, false
	case discord.ApplicationCommandOptionString:
		return o.Name, o.Description, o.Required
	case discord.ApplicationCommandOptionInt:
		return o.Name, o.Description, o.Required
	case discord.ApplicationCommandOptionBool:
		return o.Name, o.Description, o.Required
	default:
		return opt.OptionName(), "", false
	}
}

func subOptions(opt discord.ApplicationCommandOption) []discord.ApplicationCommandOption {
	if o, ok := opt.(discord.ApplicationCommandOptionSubCommand); ok {
		return o.Options
	}
	return nil
}
