package disgo

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/snowflake/v2"

	"github.com/youruser/yourproject/internal/disgo/actions"
	"github.com/youruser/yourproject/internal/disgo/commands"
	"github.com/youruser/yourproject/internal/disgo/events"
	"github.com/youruser/yourproject/internal/guards"
)

// Bot wraps the disgo client and exposes lifecycle methods.
type Bot struct {
	Client   *bot.Client
	services *guards.Services
}

// SetServices stores the cross-system Services reference.
// Called from main.go after all systems are initialized.
func (b *Bot) SetServices(svc *guards.Services) { b.services = svc }

// Services returns the cross-system Services reference.
func (b *Bot) Services() *guards.Services { return b.services }

var instance *Bot

// SetInstance stores the bot for package-level access.
// Called from main.go after NewBot().
func SetInstance(b *Bot) { instance = b }

// Instance returns the bot instance.
// Used by PocketBase hooks and actions to access the Discord client.
func Instance() *Bot { return instance }

// NewBot creates and configures the Discord bot client.
// Reads DISCORD_BOT_TOKEN from environment.
// Registers all slash commands and event listeners.
func NewBot() (*Bot, error) {
	token := os.Getenv("DISCORD_BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("disgo: DISCORD_BOT_TOKEN is not set")
	}

	// Build command handler mux
	mux := handler.New()
	allCmds := commands.All()
	for _, cmd := range allCmds {
		mux.SlashCommand("/"+cmd.Create.Name, cmd.Handler)
	}

	// Create client
	client, err := disgo.New(token,
		bot.WithDefaultGateway(),
		bot.WithGatewayConfigOpts(
			gateway.WithIntents(
				gateway.IntentGuilds,
				gateway.IntentGuildMessages,
			),
		),
		bot.WithEventListeners(mux),
	)
	if err != nil {
		return nil, fmt.Errorf("disgo: failed to create client: %w", err)
	}

	// Attach non-command event listeners
	events.RegisterAll(client)

	b := &Bot{Client: client}

	// Sync slash commands with Discord API
	if err := b.syncCommands(allCmds); err != nil {
		return nil, err
	}

	return b, nil
}

// syncCommands registers slash command definitions with the Discord API.
// If DISCORD_DEV_GUILD_ID is set, syncs to that guild (instant) instead of global (up to 1hr delay).
func (b *Bot) syncCommands(cmds []commands.Command) error {
	defs := make([]discord.ApplicationCommandCreate, len(cmds))
	for i, cmd := range cmds {
		defs[i] = cmd.Create
	}

	var guildIDs []snowflake.ID
	if devGuild := os.Getenv("DISCORD_DEV_GUILD_ID"); devGuild != "" {
		id, err := snowflake.Parse(devGuild)
		if err != nil {
			return fmt.Errorf("disgo: invalid DISCORD_DEV_GUILD_ID %q: %w", devGuild, err)
		}
		guildIDs = append(guildIDs, id)
		log.Printf("Syncing %d commands to dev guild %s", len(defs), devGuild)
	} else {
		log.Printf("Syncing %d commands globally (may take up to 1 hour to propagate)", len(defs))
	}

	return handler.SyncCommands(b.Client, defs, guildIDs)
}

// IsMember checks if a Discord user is a member of the given guild.
// Satisfies guards.DiscordService.
func (b *Bot) IsMember(guildID, userID snowflake.ID) (bool, error) {
	_, err := b.Client.Rest.GetMember(guildID, userID)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// MemberRoles returns the role IDs for a user in a guild.
// Satisfies guards.DiscordService.
func (b *Bot) MemberRoles(guildID, userID snowflake.ID) ([]snowflake.ID, error) {
	member, err := b.Client.Rest.GetMember(guildID, userID)
	if err != nil {
		return nil, err
	}
	return member.RoleIDs, nil
}

// SendNotification sends a text message to the given channel.
// Satisfies guards.DiscordService.
func (b *Bot) SendNotification(channelID snowflake.ID, content string) error {
	return actions.SendNotification(b.Client, channelID, content)
}

// CreateVoiceChannel creates a new voice channel in the given guild.
// Satisfies guards.DiscordService.
func (b *Bot) CreateVoiceChannel(guildID snowflake.ID, name string) (discord.GuildChannel, error) {
	return actions.CreateVoiceChannel(b.Client, guildID, name)
}

// OpenGateway connects to the Discord gateway. Non-blocking.
func (b *Bot) OpenGateway(ctx context.Context) error {
	return b.Client.OpenGateway(ctx)
}

// Close shuts down the gateway connection.
func (b *Bot) Close(ctx context.Context) {
	b.Client.Close(ctx)
}
