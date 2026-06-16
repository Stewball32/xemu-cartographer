// Package adminusers exposes /api/admin/users/* endpoints for the M08
// admin role + ban surface. Routes inherit RequireAuth + RequireAdmin so
// every handler can assume the caller already holds the admin role; they
// still enforce per-call guards (don't ban yourself, don't grant a missing
// slug, don't unban a non-banned user, etc.) inside the handler body.
//
// The handlers wrap `internal/roles.Grant`/`Revoke` (which already pair
// the mutation with the matching `audit_log` row) and direct
// `app.Save` calls for the ban/timeout fields paired with explicit
// `audit.Write` calls. The explicit-audit path is required for ban/
// timeout because the request payload carries a `reason` field that the
// schema-side `users_ban_transitions` hook can't see — and using
// `app.Save` from inside this handler bypasses `OnRecordUpdateRequest`,
// so we don't get duplicate audit rows from that hook.
package adminusers

import (
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/middleware"
)

// Group is the router group for /api/admin/users endpoints.
// All routes inherit RequireAuth + RequireAdmin.
var Group *router.RouterGroup[*core.RequestEvent]

var registry []func()

func register(fn func()) {
	registry = append(registry, fn)
}

// RegisterAll creates the admin/users group and registers all handlers.
func RegisterAll(se *core.ServeEvent) {
	Group = se.Router.Group("/api/admin/users")
	Group.Bind(apis.RequireAuth())
	Group.BindFunc(middleware.RequireAdmin())

	for _, fn := range registry {
		fn()
	}
}
