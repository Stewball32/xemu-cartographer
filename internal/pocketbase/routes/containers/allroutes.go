// Package containers exposes /api/admin/containers/* endpoints for managing
// xemu + browser container pairs via the podman Manager.
package containers

import (
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/middleware"
	"github.com/Stewball32/xemu-cartographer/internal/podman"
)

// Group is the router group for /api/admin/containers endpoints.
// All routes inherit RequireAuth + RequireAdmin middleware.
var Group *router.RouterGroup[*core.RequestEvent]

// Manager is the podman manager used by all container handlers.
// Injected by SetManager from cmd/server/main.go before RegisterAll runs.
var Manager *podman.Manager

var registry []func()

func register(fn func()) {
	registry = append(registry, fn)
}

// SetManager wires the podman manager. Must be called before RegisterAll.
// If the manager is nil (CONTAINERS_ENABLED=false), RegisterAll is a no-op.
func SetManager(m *podman.Manager) {
	Manager = m
}

// RegisterAll creates the containers group and registers all handlers.
// No-op when Manager is nil — keeps the server bootable without podman.
func RegisterAll(se *core.ServeEvent) {
	if Manager == nil {
		return
	}

	Group = se.Router.Group("/api/admin/containers")
	Group.Bind(apis.RequireAuth())
	Group.BindFunc(middleware.RequireAdmin())

	for _, fn := range registry {
		fn()
	}
}
