package routes

import (
	"os"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/middleware"
)

var registry []func(se *core.ServeEvent)

// register adds a route function to the registry.
// Call this from init() in each route file.
func register(fn func(se *core.ServeEvent)) {
	registry = append(registry, fn)
}

// RegisterAll wires all middleware, groups, and ungrouped routes.
// Called from cmd/server/main.go inside the OnServe hook.
func RegisterAll(se *core.ServeEvent) {
	middleware.Init(se)                    // 1. global middleware
	registerAllGroups(se)                  // 2. group packages (from allgroups.go)
	for _, fn := range registry { fn(se) } // 3. ungrouped routes

	// 4. SPA catch-all — MUST be registered last so more specific routes
	//    above take priority. Serves pb_public/ with indexFallback=true
	//    so unknown paths resolve to index.html and the SvelteKit client
	//    router can handle them.
	se.Router.GET("/{path...}", apis.Static(os.DirFS("pb_public"), true))
}
