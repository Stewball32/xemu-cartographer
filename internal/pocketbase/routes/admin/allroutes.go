package admin

import (
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/middleware"
)

// Group is the router group for /api/admin endpoints.
// All routes registered on this group inherit RequireAuth + RequireAdmin middleware.
var Group *router.RouterGroup[*core.RequestEvent]

var registry []func()

func register(fn func()) {
	registry = append(registry, fn)
}

// RegisterAll creates the admin group and registers all admin routes.
func RegisterAll(se *core.ServeEvent) {
	Group = se.Router.Group("/api/admin")
	Group.Bind(apis.RequireAuth())
	Group.BindFunc(middleware.RequireAdmin())

	for _, fn := range registry {
		fn()
	}
}
