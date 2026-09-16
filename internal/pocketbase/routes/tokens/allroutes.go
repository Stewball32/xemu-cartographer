// Package tokens exposes /api/admin/tokens — minting, listing and revoking
// api_tokens rows (machine / spectator / device keys; design §6.3).
//
// Unlike the other /api/admin/* groups this one binds neither
// apis.RequireAuth nor middleware.RequireAdmin: a machine key holding
// token.* may call it, and a PB JWT is only one of the carriers. Every
// handler instead resolves + authorises through pb.Check with the exact
// action (token.mint / overlay.mint, token.list, token.revoke), so an
// unauthenticated caller gets 401 and an under-scoped one 403.
package tokens

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

// Group is the router group for /api/admin/tokens endpoints.
var Group *router.RouterGroup[*core.RequestEvent]

var registry []func()

func register(fn func()) {
	registry = append(registry, fn)
}

// RegisterAll creates the admin/tokens group and registers all handlers.
// routes/allgroups.go calls it (R-16).
func RegisterAll(se *core.ServeEvent) {
	Group = se.Router.Group("/api/admin/tokens")

	for _, fn := range registry {
		fn()
	}
}
