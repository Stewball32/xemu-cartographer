package containers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/pocketbase/pocketbase/core"

	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/podman"
)

func init() {
	register(func() {
		// GET /api/admin/containers — list all managed container pairs.
		Group.GET("", func(e *core.RequestEvent) error {
			list, err := Manager.List()
			if err != nil {
				return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return e.JSON(http.StatusOK, list)
		})

		// POST /api/admin/containers — create a new container pair.
		// Optional "game_iso" attaches a game ISO from the shared library
		// (Config.ISODir) as the instance's DVD, so it boots straight into
		// that game. Restricted to a bare filename (no path separators) — the
		// API can only pick from the library, never an arbitrary host path.
		Group.POST("", func(e *core.RequestEvent) error {
			var body struct {
				Name    string `json:"name"`
				GameISO string `json:"game_iso"`
			}
			if err := e.BindBody(&body); err != nil || body.Name == "" {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
			}
			if body.GameISO != "" && (strings.ContainsAny(body.GameISO, `/\`) || body.GameISO == ".." || strings.HasPrefix(body.GameISO, ".")) {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "game_iso must be a bare filename in the ISO library"})
			}
			// Namespace the container under the deployment's prefix (empty in prod).
			// Idempotent: an operator may type the bare or already-prefixed name.
			if p := Manager.NamePrefix(); p != "" && !strings.HasPrefix(body.Name, p) {
				body.Name = p + body.Name
			}
			info, err := Manager.CreateWithOptions(body.Name, podman.CreateOptions{GameISO: body.GameISO})
			if err != nil {
				return e.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
			}
			return e.JSON(http.StatusCreated, info)
		})

		// GET /api/admin/containers/{name} — fetch live podman status.
		Group.GET("/{name}", func(e *core.RequestEvent) error {
			name := e.Request.PathValue("name")
			status, err := Manager.Status(name)
			if err != nil {
				return e.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
			}
			return e.JSON(http.StatusOK, map[string]string{"status": status})
		})

		// POST /api/admin/containers/{name}/start — start xemu + browser.
		Group.POST("/{name}/start", func(e *core.RequestEvent) error {
			name := e.Request.PathValue("name")
			if err := Manager.Start(name); err != nil {
				return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return e.NoContent(http.StatusNoContent)
		})

		// POST /api/admin/containers/{name}/stop — stop xemu + browser.
		Group.POST("/{name}/stop", func(e *core.RequestEvent) error {
			name := e.Request.PathValue("name")
			if err := Manager.Stop(name); err != nil {
				return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return e.NoContent(http.StatusNoContent)
		})

		// DELETE /api/admin/containers/{name} — remove the pair.
		Group.DELETE("/{name}", func(e *core.RequestEvent) error {
			name := e.Request.PathValue("name")
			if err := Manager.Remove(name); err != nil {
				return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return e.NoContent(http.StatusNoContent)
		})

		// GET /api/admin/containers/{name}/detail — combined info + status + scraper state.
		Group.GET("/{name}/detail", func(e *core.RequestEvent) error {
			name := e.Request.PathValue("name")
			info, ok := Manager.Get(name)
			if !ok {
				return e.JSON(http.StatusNotFound, map[string]string{"error": "container not found"})
			}
			status, _ := Manager.Status(name)

			var scraperState *scraperiface.InstanceState
			if Services != nil && Services.Scraper != nil {
				if st, ok := Services.Scraper.InstanceState(name); ok {
					scraperState = &st
				}
			}

			return e.JSON(http.StatusOK, map[string]any{
				"info":    info,
				"status":  status,
				"scraper": scraperState,
			})
		})

		// GET /api/admin/containers/{name}/logs?which=xemu|browser&tail=N
		Group.GET("/{name}/logs", func(e *core.RequestEvent) error {
			name := e.Request.PathValue("name")
			which := e.Request.URL.Query().Get("which")
			if which != "xemu" && which != "browser" {
				which = "xemu"
			}
			tail := 200
			if t := e.Request.URL.Query().Get("tail"); t != "" {
				if n, err := strconv.Atoi(t); err == nil {
					tail = n
				}
			}
			logs, err := Manager.Logs(name, tail, which)
			if err != nil {
				return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error(), "logs": logs})
			}
			return e.JSON(http.StatusOK, map[string]any{
				"logs":  logs,
				"which": which,
			})
		})
	})
}
