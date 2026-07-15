package commands

import (
	"fmt"
	"os"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"

	"github.com/Stewball32/xemu-cartographer/internal/gamertags"
	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
)

// /mybox — a player's own xemu container, driven from Discord.
//
// A Discord user is mapped to a cartographer account via the Discord OAuth link
// (svc.PB.FindUserByDiscordID), then to their gamertag(s), then matched against
// the live container rosters (Scraper.Membership) — the same resolution the
// /api/play "my match" page uses. Replies are ephemeral (only the caller sees
// their own box).
//
// READY now (read-only, use the wired Services): `status`, `link`.
// STUBBED (need the podman provisioner/manager surfaced through Services —
// see the TODO in the request/stop handlers): `request`, `stop`.
func init() {
	register(Command{
		Create: discord.SlashCommandCreate{
			Name:        "mybox",
			Description: "Your xemu container",
			Options: []discord.ApplicationCommandOption{
				discord.ApplicationCommandOptionSubCommand{
					Name:        "status",
					Description: "Show the container you're currently matched to",
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "link",
					Description: "Get the link to play your matched container",
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "request",
					Description: "Spin up a container for a game (coming soon)",
					Options: []discord.ApplicationCommandOption{
						discord.ApplicationCommandOptionString{
							Name:        "game",
							Description: "Which game to boot",
						},
					},
				},
				discord.ApplicationCommandOptionSubCommand{
					Name:        "stop",
					Description: "Tear down your container (coming soon)",
				},
			},
		},
		Handler: handleMybox,
	})
}

func handleMybox(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	sub := ""
	if data.SubCommandName != nil {
		sub = *data.SubCommandName
	}
	switch sub {
	case "status":
		return handleMyboxStatus(data, e)
	case "link":
		return handleMyboxLink(data, e)
	case "request":
		return handleMyboxRequest(data, e)
	case "stop":
		return handleMyboxStop(data, e)
	default:
		return replyEphemeral(e, "Unknown subcommand.")
	}
}

// handleMyboxStatus — READY. Resolve the caller's live container and report it.
func handleMyboxStatus(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	container, res := resolveCallerContainer(e.User().ID.String())
	return replyEmbedEphemeral(e, myboxStatusEmbed(container, res))
}

// handleMyboxLink — READY. Give the caller a play link for their container. The
// /play page re-resolves the caller's gamertag server-side, so the link is the
// same for everyone; it just needs the public base URL (PUBLIC_APP_URL).
func handleMyboxLink(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	container, res := resolveCallerContainer(e.User().ID.String())
	base := os.Getenv("PUBLIC_APP_URL")
	if base == "" {
		return replyEphemeral(e, "The play link isn't configured (set PUBLIC_APP_URL). Ask an admin.")
	}
	if res != resolveMatched {
		return replyEphemeral(e, "You're not in a live match right now — nothing to open. Log in and join a container first: "+base+"/play/")
	}
	return replyEphemeral(e, fmt.Sprintf("Open your box (**%s**): %s/play/", container, base))
}

// handleMyboxRequest — STUB. Spinning up a container needs the podman
// provisioner, which is NOT on guards.Services today (it lives in the
// containers/play route packages). TODO: surface a small provisioner interface
// on Services (mirroring scraperiface) and call it here — resolve the caller's
// account, pick the ISO by `game`, provision + start, reply with the link.
func handleMyboxRequest(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	game, _ := data.OptString("game")
	msg := "Spinning up containers from Discord isn't wired yet."
	if game != "" {
		msg = fmt.Sprintf("Spinning up a **%s** box from Discord isn't wired yet.", game)
	}
	return replyEphemeral(e, msg+" For now, request one on the web /play page.")
}

// handleMyboxStop — STUB. Same reason as request: teardown needs the podman
// manager surfaced through Services. TODO: resolve the caller's container and
// call Manager.Stop/Remove through a Services interface.
func handleMyboxStop(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	return replyEphemeral(e, "Stopping your container from Discord isn't wired yet. For now, tear it down on the web /play page.")
}

// resolveResult is the outcome of mapping a Discord user to a live container.
type resolveResult int

const (
	resolveMatched     resolveResult = iota // found a live container for the caller
	resolveIdle                             // linked account, but not in any live roster
	resolveNotLinked                        // Discord account isn't linked to a cartographer user
	resolveUnavailable                      // Services (PB/scraper) not wired
)

// resolveCallerContainer maps the invoking Discord user → cartographer account →
// gamertags → the live container whose roster contains one of those gamertags.
// Mirrors the /api/play resolveCaller flow, fail-soft (any lookup miss → idle).
func resolveCallerContainer(discordID string) (string, resolveResult) {
	s := services()
	if s == nil || s.PB == nil || s.App == nil || s.Scraper == nil {
		return "", resolveUnavailable
	}
	user, err := s.PB.FindUserByDiscordID(discordID)
	if err != nil || user == nil {
		return "", resolveNotLinked
	}
	tags, err := gamertags.SanitizedForUser(s.App, user.Id)
	if err != nil {
		return "", resolveIdle // fail-soft, matching resolveCaller
	}
	container, ok := scraperiface.MatchContainer(s.Scraper.Membership(), tags)
	if !ok {
		return "", resolveIdle
	}
	return container, resolveMatched
}

// myboxStatusEmbed renders the caller's container status. Pure (no interaction /
// Services), so it's unit-testable — the resolution above is the thin live part.
func myboxStatusEmbed(container string, res resolveResult) discord.Embed {
	const blurple = 0x5865f2
	switch res {
	case resolveMatched:
		return discord.Embed{
			Title:       "Your box",
			Color:       blurple,
			Description: fmt.Sprintf("You're matched to **%s**. Use `/mybox link` to open it.", container),
		}
	case resolveIdle:
		return discord.Embed{
			Title:       "Your box",
			Color:       blurple,
			Description: "You're not in a live match right now. Join a container (or `/mybox request`) to get playing.",
		}
	case resolveNotLinked:
		return discord.Embed{
			Title:       "Not linked",
			Color:       0xed4245,
			Description: "Your Discord account isn't linked to a cartographer account yet. Sign in with Discord on the web app first.",
		}
	default: // resolveUnavailable
		return discord.Embed{
			Title:       "Unavailable",
			Color:       0xed4245,
			Description: "The container service isn't running right now.",
		}
	}
}
