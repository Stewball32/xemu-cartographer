// Package rosters exposes /api/rosters/* endpoints that mutate a roster row's
// active state — self-leave + owner/manager-driven removal. PB rules on the
// rosters collection (set in schema/rosters.go) gate full CRUD to the team's
// `created_by` or admin; these routes carve out specific paths that
// captains/managers + the gamertag owner can drive.
package rosters

import (
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

// Group is the router group for /api/rosters endpoints.
var Group *router.RouterGroup[*core.RequestEvent]

var registry []func()

func register(fn func()) {
	registry = append(registry, fn)
}

// RegisterAll creates the group and registers all handlers.
func RegisterAll(se *core.ServeEvent) {
	Group = se.Router.Group("/api/rosters")
	Group.Bind(apis.RequireAuth())

	for _, fn := range registry {
		fn()
	}
}
