package containers

import (
	"net/http"

	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(func() {
		// DELETE /api/admin/containers/{name}/files — remove the on-disk
		// bind-mount source directories (xemu configs/<name> and browser
		// config-<name>). Independent from DELETE /{name} (which removes the
		// podman containers + manager record). Refuses if either container
		// half is currently live so we don't yank the rug out from under a
		// running X session.
		Group.DELETE("/{name}/files", func(e *core.RequestEvent) error {
			name := e.Request.PathValue("name")
			if err := Manager.DeleteFiles(name); err != nil {
				return e.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
			}
			return e.NoContent(http.StatusNoContent)
		})

		// POST /api/admin/containers/cleanup — remove on-disk artefacts for
		// containers no longer tracked by the manager (orphans). Baseline
		// HDDs whose basename starts with "_" are preserved.
		Group.POST("/cleanup", func(e *core.RequestEvent) error {
			deleted, err := Manager.CleanupOrphans()
			if err != nil {
				return e.JSON(http.StatusInternalServerError, map[string]any{
					"error":   err.Error(),
					"deleted": deleted,
				})
			}
			return e.JSON(http.StatusOK, map[string]any{"deleted": deleted})
		})
	})
}
