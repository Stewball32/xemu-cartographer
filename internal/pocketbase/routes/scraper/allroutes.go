// Package scraper exposes /api/admin/scraper/* endpoints for managing
// running game-scraper instances via internal/scraper/manager.
//
// Mirrors the routes/containers/ pattern: a package-level Group + Manager,
// SetManager() called from cmd/server/main.go before RegisterAll, and
// init()-registered handler functions in handlers.go.
package scraper

import (
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/middleware"
)

// Group is the router group for /api/admin/scraper endpoints.
// All routes inherit RequireAuth + RequireAdmin middleware.
var Group *router.RouterGroup[*core.RequestEvent]

// Manager is the scraper manager used by all handlers.
// Injected by SetManager from cmd/server/main.go before RegisterAll runs.
// Typed against the Service interface (not the concrete *manager.Manager) so
// this package has no compile-time dependency on internal/scraper/manager.
var Manager scraperiface.Service

var registry []func()

func register(fn func()) {
	registry = append(registry, fn)
}

// SetManager wires the scraper manager. Must be called before RegisterAll.
// If the manager is nil, RegisterAll is a no-op — keeps the server bootable
// without a scraper subsystem (e.g. for early development or stripped builds).
func SetManager(m scraperiface.Service) {
	Manager = m
}

// RegisterAll creates the scraper group and registers all handlers.
// No-op when Manager is nil.
func RegisterAll(se *core.ServeEvent) {
	if Manager == nil {
		return
	}

	Group = se.Router.Group("/api/admin/scraper")
	Group.Bind(apis.RequireAuth())
	Group.BindFunc(middleware.RequireAdmin())

	for _, fn := range registry {
		fn()
	}
}
