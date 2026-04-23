package hooks

import (
	"github.com/pocketbase/pocketbase"
	"github.com/Stewball32/xemu-cartographer/internal/guards"
)

var registry []func(app *pocketbase.PocketBase)

var svc *guards.Services

// SetServices stores the cross-system Services reference.
// Called from main.go after all systems are initialized.
func SetServices(s *guards.Services) { svc = s }

// register adds a hook function to the registry.
// Call this from init() in each domain file.
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
