package commands

import (
	"fmt"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/omit"
	"github.com/disgoorg/snowflake/v2"

	"github.com/Stewball32/xemu-cartographer/internal/discordcfg"
)

// /setup — admin-gated, idempotent guild provisioning for the container-lifecycle
// integration. Creates a "Cartographer" category + one channel per bot function
// and PERSISTS the resulting IDs to the discord_guilds row (channel→hook
// routing), so commands/events know where to post and re-running never
// restructures the guild. NEVER runs on join — only when an admin invokes it, so
// a prod guild is always safe.
//
// Reuse-by-ID-then-by-name makes it safe to re-run and to adopt channels that
// already exist (e.g. created out-of-band). Manage-Server gated by default.
func init() {
	register(Command{
		Create: discord.SlashCommandCreate{
			Name:                     "setup",
			Description:              "Provision this server's Cartographer channels (admin)",
			DefaultMemberPermissions: omit.NewPtr(discord.PermissionManageGuild),
		},
		Handler: handleSetup,
	})
}

// setupCategoryName is the category the bot channels live under.
const setupCategoryName = "Cartographer"

// channelSpec is one provisioned channel: the Discord channel name and where its
// ID is stored in the guild's Channels binding.
type channelSpec struct {
	name   string
	assign func(*discordcfg.Channels, string)
}

// setupChannelSpec is the fixed set of function channels /setup provisions. Pure
// (no Discord/DB), so the shape is unit-testable.
func setupChannelSpec() []channelSpec {
	return []channelSpec{
		{"container-status", func(c *discordcfg.Channels, id string) { c.ContainerStatus = id }},
		{"kiosk-links", func(c *discordcfg.Channels, id string) { c.KioskLinks = id }},
		{"announcements", func(c *discordcfg.Channels, id string) { c.Announcements = id }},
		{"bot-log", func(c *discordcfg.Channels, id string) { c.BotLog = id }},
	}
}

func handleSetup(_ discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	gid := e.GuildID()
	if gid == nil {
		return replyEphemeral(e, "Run `/setup` inside a server.")
	}
	app := commandApp()
	if app == nil {
		return replyEphemeral(e, "Setup is unavailable right now (PocketBase not wired).")
	}

	// Existing guild channels — the basis for idempotent reuse-by-name.
	existing, err := e.Client().Rest.GetGuildChannels(*gid)
	if err != nil {
		return replyEphemeral(e, "Couldn't read this server's channels — does the bot have Manage Channels?")
	}

	// Seed from anything already persisted so re-runs prefer stored IDs.
	ch, _ := discordcfg.GetChannels(app, gid.String())
	var log []string

	catID, made, err := ensureCategory(e, *gid, existing, ch.Category)
	if err != nil {
		return replyEphemeral(e, "Failed to create the category: "+err.Error())
	}
	ch.Category = catID.String()
	log = append(log, statusLine(setupCategoryName, made))

	for _, spec := range setupChannelSpec() {
		id, made, err := ensureTextChannel(e, *gid, existing, spec.name, catID)
		if err != nil {
			return replyEphemeral(e, fmt.Sprintf("Failed to create #%s: %s", spec.name, err.Error()))
		}
		spec.assign(&ch, id.String())
		log = append(log, statusLine("#"+spec.name, made))
	}

	if err := discordcfg.SetChannels(app, gid.String(), ch); err != nil {
		return replyEphemeral(e, "Channels are ready but saving the config failed: "+err.Error())
	}

	emb := discord.Embed{
		Title:       "Cartographer setup",
		Color:       0x57f287,
		Description: "Provisioned + saved this server's channel routing:\n" + strings.Join(log, "\n"),
	}
	return replyEmbedEphemeral(e, emb)
}

// ensureCategory returns the category ID, preferring a still-valid persisted id,
// then a category matching setupCategoryName, else creating one. made=true when
// created.
func ensureCategory(e *handler.CommandEvent, gid snowflake.ID, existing []discord.GuildChannel, storedID string) (snowflake.ID, bool, error) {
	if id, ok := findChannel(existing, storedID, "", discord.ChannelTypeGuildCategory); ok {
		return id, false, nil
	}
	if id, ok := findChannelByName(existing, setupCategoryName, discord.ChannelTypeGuildCategory); ok {
		return id, false, nil
	}
	c, err := e.Client().Rest.CreateGuildChannel(gid, discord.GuildCategoryChannelCreate{Name: setupCategoryName})
	if err != nil {
		return 0, false, err
	}
	return c.ID(), true, nil
}

// ensureTextChannel returns the ID of the named text channel under parent,
// reusing an existing one (by name) or creating it. made=true when created.
func ensureTextChannel(e *handler.CommandEvent, gid snowflake.ID, existing []discord.GuildChannel, name string, parent snowflake.ID) (snowflake.ID, bool, error) {
	if id, ok := findChannelByName(existing, name, discord.ChannelTypeGuildText); ok {
		return id, false, nil
	}
	c, err := e.Client().Rest.CreateGuildChannel(gid, discord.GuildTextChannelCreate{
		Name:     name,
		ParentID: parent,
	})
	if err != nil {
		return 0, false, err
	}
	return c.ID(), true, nil
}

// findChannel looks up a channel by (non-empty) stored ID string and type.
func findChannel(chans []discord.GuildChannel, storedID string, _ string, typ discord.ChannelType) (snowflake.ID, bool) {
	if storedID == "" {
		return 0, false
	}
	id, err := snowflake.Parse(storedID)
	if err != nil {
		return 0, false
	}
	for _, c := range chans {
		if c.ID() == id && c.Type() == typ {
			return id, true
		}
	}
	return 0, false
}

// findChannelByName matches a channel by case-insensitive name + type.
func findChannelByName(chans []discord.GuildChannel, name string, typ discord.ChannelType) (snowflake.ID, bool) {
	for _, c := range chans {
		if c.Type() == typ && strings.EqualFold(c.Name(), name) {
			return c.ID(), true
		}
	}
	return 0, false
}

func statusLine(name string, created bool) string {
	if created {
		return "• " + name + " — created"
	}
	return "• " + name + " — reused"
}
