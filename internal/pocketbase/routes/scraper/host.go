package scraper

import (
	"net/http"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/hostrunner"
)

// HostControl is the arbitration surface the host-runner control endpoints call.
// Implemented by *hostrunner.Registry. Kept as an interface so this package has
// no compile-time dependency on the runner wiring — SetHostControl injects the
// live registry from cmd/server/main.go once the host-runner subsystem is built.
type HostControl interface {
	Status(instance string) hostrunner.Status
	SetAuthority(instance string, auth hostrunner.Authority) bool
}

// HostRunners is the injected control surface (nil until wired — endpoints then
// return 503, keeping the server bootable without the host-runner subsystem).
var HostRunners HostControl

// SetHostControl wires the host-runner registry. Call before RegisterAll.
func SetHostControl(c HostControl) { HostRunners = c }

func init() {
	register(func() {
		// GET /api/admin/scraper/{name}/host — arbitration + last-activity view
		// (authority, current screen, native start counts, last emitted keys).
		Group.GET("/{name}/host", func(e *core.RequestEvent) error {
			if HostRunners == nil {
				return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "host-runner subsystem not enabled"})
			}
			name := e.Request.PathValue("name")
			if name == "" {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
			}
			return e.JSON(http.StatusOK, HostRunners.Status(name))
		})

		// POST /api/admin/scraper/{name}/host — arbitration control (requirement
		// 4). Declarative: body {"authority":"runner"|"admin"|"disabled"}.
		//   admin    = take control (pause the runner; the admin's keysyms drive
		//              the SAME vncinput channel natively — no takeover plumbing)
		//   runner   = release control back to the auto-host runner
		//   disabled = turn auto-hosting off
		// 200 with the new Status; 404 when no runner is attached for the name.
		Group.POST("/{name}/host", func(e *core.RequestEvent) error {
			if HostRunners == nil {
				return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "host-runner subsystem not enabled"})
			}
			name := e.Request.PathValue("name")
			if name == "" {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
			}
			var body struct {
				Authority string `json:"authority"`
			}
			if err := e.BindBody(&body); err != nil {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			auth, ok := hostrunner.ParseAuthority(body.Authority)
			if !ok {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "authority must be one of: runner, admin, disabled"})
			}
			if !HostRunners.SetAuthority(name, auth) {
				return e.JSON(http.StatusNotFound, map[string]string{"error": "no host runner attached for " + name})
			}
			return e.JSON(http.StatusOK, HostRunners.Status(name))
		})
	})
}
