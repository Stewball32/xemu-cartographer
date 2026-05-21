package hooks

import (
	"github.com/Stewball32/xemu-cartographer/internal/guards"
	"github.com/pocketbase/pocketbase"
)

var registry []func(app *pocketbase.PocketBase)

//nolint:unused // registration scaffolding for the documented hook pattern (CLAUDE.md)
var svc *guards.Services

// SetServices stores the cross-system Services reference.
// Called from main.go after all systems are initialized.
func SetServices(s *guards.Services) { svc = s }

//nolint:unused // registration scaffolding for the documented hook pattern (CLAUDE.md)
func register(fn func(app *pocketbase.PocketBase)) {
	registry = append(registry, fn)
}

// RegisterAll wires all record lifecycle hooks.
// Called from cmd/server/main.go.
func RegisterAll(app *pocketbase.PocketBase) {
	for _, fn := range registry {
		fn(app)
	}
}
