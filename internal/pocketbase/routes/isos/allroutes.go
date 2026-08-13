// Package isos exposes /api/admin/isos/* — the ISO/game-catalog management
// surface (inbox scan + ingest, metadata/server_iso/offset_set edits, delete).
//
// Access is ORGANIZER-OR-ADMIN: the catalog is managed from the organizer-gated
// /organizer/games page, matching the isos collection's own PB rules
// (organizer-or-admin create/delete). The route path keeps its historical
// /api/admin/ prefix — renaming it would churn every client for no behavioral
// gain. The player-scoped picker lives in routes/play (GET /api/play/isos +
// POST /api/play/request); the admin kiosk + VNC path (routes/containers) is
// untouched.
//
// Under the ingest model the catalog is not a scan of arbitrary files: each
// row OWNS a managed <record-id>.iso produced by the inbox→ingest pipeline
// (internal/isoingest), so no podman manager is needed here at all.
package isos

import (
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/Stewball32/xemu-cartographer/internal/roles"
)

// Group is the router group for /api/admin/isos endpoints.
// All routes inherit RequireAuth + the organizer-or-admin gate below.
var Group *router.RouterGroup[*core.RequestEvent]

var registry []func()

func register(fn func()) { registry = append(registry, fn) }

// requireOrganizerOrAdmin admits superusers, admins (roles.IsAdminAuth), and
// holders of the organizer role — the same population the isos collection's PB
// rules admit for mutations.
func requireOrganizerOrAdmin() func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if e.Auth == nil {
			return apis.NewUnauthorizedError("authentication required", nil)
		}
		if roles.IsAdminAuth(e.App, e.Auth) {
			return e.Next()
		}
		if ok, err := roles.Has(e.App, e.Auth.Id, "organizer"); err == nil && ok {
			return e.Next()
		}
		return apis.NewForbiddenError("organizer or admin role required", nil)
	}
}

// RegisterAll creates the isos group and registers all handlers. Always active —
// the catalog + ingest pipeline need only PocketBase and the configured dirs.
func RegisterAll(se *core.ServeEvent) {
	Group = se.Router.Group("/api/admin/isos")
	Group.Bind(apis.RequireAuth())
	Group.BindFunc(requireOrganizerOrAdmin())

	for _, fn := range registry {
		fn()
	}
}
