// Package team_membership_requests exposes /api/team-membership-requests/*
// endpoints that resolve pending invites + join requests. Creation lives in
// the sibling teams/ package (POST /api/teams/{teamId}/invite +
// /api/teams/{teamId}/join-requests); this package owns transitions out of
// `pending` — accept, decline, cancel.
//
// All routes require an authenticated caller. Per-handler logic verifies
// the caller is the appropriate party (recipient for decline, initiator
// for cancel, recipient OR owner/manager-depending-on-direction for
// accept) before mutating.
package team_membership_requests

import (
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

// Group is the router group for /api/team-membership-requests endpoints.
var Group *router.RouterGroup[*core.RequestEvent]

var registry []func()

func register(fn func()) {
	registry = append(registry, fn)
}

// RegisterAll creates the group and registers all handlers.
func RegisterAll(se *core.ServeEvent) {
	Group = se.Router.Group("/api/team-membership-requests")
	Group.Bind(apis.RequireAuth())

	for _, fn := range registry {
		fn()
	}
}
