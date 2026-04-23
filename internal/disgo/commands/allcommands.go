package commands

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/youruser/yourproject/internal/guards"
)

// Command pairs a slash command definition with its handler.
type Command struct {
	Create  discord.SlashCommandCreate
	Handler handler.SlashCommandHandler
}

var registry []Command

var svc *guards.Services

// SetServices stores the cross-system Services reference.
// Called from main.go after all systems are initialized.
func SetServices(s *guards.Services) { svc = s }

// services returns the cross-system Services reference for use in command handlers.
func services() *guards.Services { return svc }

// register adds a command to the registry.
// Call this from init() in each command file.
func register(cmd Command) {
	registry = append(registry, cmd)
}

// All returns all registered commands.
// Called by bot.go to build the handler mux and sync with Discord.
func All() []Command {
	return registry
}
