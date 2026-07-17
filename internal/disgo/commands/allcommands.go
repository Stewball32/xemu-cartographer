package commands

import (
	"github.com/Stewball32/xemu-cartographer/internal/guards"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

// Command pairs a slash command definition with its handler.
type Command struct {
	Create  discord.SlashCommandCreate
	Handler handler.SlashCommandHandler
}

var registry []Command

//nolint:unused // registration scaffolding for the documented command pattern (CLAUDE.md)
var svc *guards.Services

// SetServices stores the cross-system Services reference.
// Called from main.go after all systems are initialized.
func SetServices(s *guards.Services) { svc = s }

//nolint:unused // accessor for the registration scaffolding above
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

// componentRegistry holds message-component (button / select) route
// registrations. Each entry binds its handler onto the mux — components carry
// no slash-command definition to sync, only a custom_id route (e.g. the
// /config tags multi-select). Populated via registerComponent from init().
var componentRegistry []func(*handler.Mux)

// registerComponent adds a component route binder to the registry.
// Call this from init() in a command file that owns a component.
func registerComponent(fn func(*handler.Mux)) {
	componentRegistry = append(componentRegistry, fn)
}

// BindComponents binds every registered component route onto the mux.
// Called by bot.go alongside the slash-command handlers.
func BindComponents(m *handler.Mux) {
	for _, fn := range componentRegistry {
		fn(m)
	}
}
