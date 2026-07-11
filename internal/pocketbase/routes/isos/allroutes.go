// Package isos exposes /api/admin/isos/* — the admin-scoped CRUD surface for the
// game-ISO library players pick from when they request an instance.
//
// The library is a CATALOG over the shared ISO dir on the host (podman
// Config.ISODir): each `isos` record names a bare filename in that dir plus
// player-facing metadata. Admins manage the catalog here; the player-scoped
// picker lives in routes/play (GET /api/play/isos + POST /api/play/request).
//
// Modeled on routes/containers + routes/scraper: a package-level Group + a
// SetManager-injected podman Manager, init()-registered handlers. The podman
// Manager is OPTIONAL — the collection CRUD needs only PocketBase, so this group
// registers whenever the server boots (even with CONTAINERS_ENABLED=false, so an
// operator can pre-populate the catalog). The Manager, when wired, powers the
// library scan (GET /library) and the "does this file actually exist" guard on
// create/update.
//
// The admin kiosk + VNC path (routes/containers) is untouched — this is a new,
// additive collection + route.
package isos

import (
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/middleware"
	"github.com/Stewball32/xemu-cartographer/internal/podman"
)

// Group is the router group for /api/admin/isos endpoints.
// All routes inherit RequireAuth + RequireAdmin middleware.
var Group *router.RouterGroup[*core.RequestEvent]

// Manager is the podman manager used for the library scan + filename-existence
// validation. Injected by SetManager from cmd/server/main.go before RegisterAll.
// Nil is fine — the collection CRUD works without it; only /library and the
// on-disk existence guard degrade (503 / skipped).
var Manager *podman.Manager

var registry []func()

func register(fn func()) { registry = append(registry, fn) }

// SetManager wires the podman manager (optional). Call before RegisterAll.
func SetManager(m *podman.Manager) { Manager = m }

// RegisterAll creates the isos group and registers all handlers. Always active
// (the catalog is a plain PocketBase collection); the Manager only gates the
// library-scan + existence-check extras.
func RegisterAll(se *core.ServeEvent) {
	Group = se.Router.Group("/api/admin/isos")
	Group.Bind(apis.RequireAuth())
	Group.BindFunc(middleware.RequireAdmin())

	for _, fn := range registry {
		fn()
	}
}
