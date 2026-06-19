// Package overlays exposes /api/overlay-tokens/* — minting, listing, and
// revoking the M10 scoped read-only overlay tokens used by OBS browser-source
// overlays. All routes RequireAuth; each handler additionally checks the
// "manage overlays" permission (admin/superuser inherently, or the
// overlay_manager role) via canManageOverlays. The render surface (the actual
// overlay pages) stays deferred — this is the auth layer only.
package overlays

import (
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/Stewball32/xemu-cartographer/internal/overlaytoken"
	"github.com/Stewball32/xemu-cartographer/internal/roles"
)

// Group is the router group for /api/overlay-tokens. RequireAuth-bound;
// per-handler permission via canManageOverlays.
var Group *router.RouterGroup[*core.RequestEvent]

// DefaultTTL is the lifetime applied when a mint request omits ttl_seconds.
// Defaults to the long-lived overlaytoken.DefaultTTL; main.go may override it
// from OVERLAY_TOKEN_TTL_HOURS via SetDefaultTTL.
var DefaultTTL = overlaytoken.DefaultTTL

// SetDefaultTTL overrides the default token lifetime (called from main.go).
func SetDefaultTTL(d time.Duration) {
	if d > 0 {
		DefaultTTL = d
	}
}

var registry []func()

func register(fn func()) { registry = append(registry, fn) }

// RegisterAll creates the overlay-tokens group and registers its handlers.
func RegisterAll(se *core.ServeEvent) {
	Group = se.Router.Group("/api/overlay-tokens")
	Group.Bind(apis.RequireAuth())
	for _, fn := range registry {
		fn()
	}
}

// canManageOverlays gates minting/revoking: a superuser or admin-role holder
// inherently, OR a holder of the dedicated overlay_manager role (a non-admin
// "stream helper"). The "both ways" permission model Stewart chose.
func canManageOverlays(app core.App, auth *core.Record) bool {
	if auth == nil {
		return false
	}
	if roles.IsAdminAuth(app, auth) {
		return true
	}
	ok, err := roles.Has(app, auth.Id, "overlay_manager")
	return err == nil && ok
}
