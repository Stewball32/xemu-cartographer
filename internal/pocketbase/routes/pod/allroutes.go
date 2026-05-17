package pod

import (
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/middleware"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

// Group is the router group for /api/pod endpoints.
// All routes registered on this group inherit RequireAuth + RequireAdmin middleware.
var Group *router.RouterGroup[*core.RequestEvent]

var registry []func()

//nolint:unused // registration scaffolding for the documented route pattern (CLAUDE.md)
func register(fn func()) {
	registry = append(registry, fn)
}

// RegisterAll creates the pod group and registers all pod routes.
func RegisterAll(se *core.ServeEvent) {
	Group = se.Router.Group("/api/pod")
	Group.Bind(apis.RequireAuth())
	Group.BindFunc(middleware.RequireAdmin())

	for _, fn := range registry {
		fn()
	}
}
