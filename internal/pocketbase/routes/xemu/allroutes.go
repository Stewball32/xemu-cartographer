// Package xemu exposes /api/admin/xemu/* endpoints for smoke-testing the
// memory bridge against a running xemu instance. In-process they open the
// QMP socket themselves; in wire mode (XC_SCRAPER_URL set) each GET is
// relayed to the daemon's POST /api/ctl/xemu/{tool} — see proxy.go.
package xemu

import (
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/middleware"
)

// Group is the router group for /api/admin/xemu endpoints.
// All routes inherit RequireAuth + RequireAdmin middleware.
var Group *router.RouterGroup[*core.RequestEvent]

var registry []func()

func register(fn func()) {
	registry = append(registry, fn)
}

// RegisterAll creates the xemu group and registers all handlers.
func RegisterAll(se *core.ServeEvent) {
	Group = se.Router.Group("/api/admin/xemu")
	Group.Bind(apis.RequireAuth())
	Group.BindFunc(middleware.RequireAdmin(authz.ActionAdminXemu))

	for _, fn := range registry {
		fn()
	}
}
