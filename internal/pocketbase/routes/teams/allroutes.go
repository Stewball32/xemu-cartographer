// Package teams exposes /api/teams/* endpoints that orchestrate
// team-membership flows (invite / join-request creation) landed in M23c.
// Acceptance / decline / cancel routes live under
// /api/team-membership-requests/* (a sibling package); roster
// mutations live under /api/rosters/* (a third sibling).
//
// All routes here require an authenticated caller; per-handler logic
// checks owner/manager status, target identity, and pending state via
// internal/teamperms before mutating.
package teams

import (
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

// Group is the router group for /api/teams endpoints. All routes inherit
// RequireAuth — anything that needs owner/manager privilege checks that
// inside its own handler.
var Group *router.RouterGroup[*core.RequestEvent]

var registry []func()

func register(fn func()) {
	registry = append(registry, fn)
}

// RegisterAll creates the teams group and registers all handlers.
func RegisterAll(se *core.ServeEvent) {
	Group = se.Router.Group("/api/teams")
	Group.Bind(apis.RequireAuth())

	for _, fn := range registry {
		fn()
	}
}
