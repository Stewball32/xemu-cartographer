package containers

import (
	"net/http"

	"github.com/pocketbase/pocketbase/core"
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
		Group.POST("", func(e *core.RequestEvent) error {
			var body struct {
				Name string `json:"name"`
			}
			if err := e.BindBody(&body); err != nil || body.Name == "" {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
			}
			info, err := Manager.Create(body.Name)
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
	})
}
