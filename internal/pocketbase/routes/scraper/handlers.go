package scraper

import (
	"errors"
	"net/http"

	"github.com/pocketbase/pocketbase/core"

	scrapermgr "github.com/Stewball32/xemu-cartographer/internal/scraper/manager"
)

func init() {
	register(func() {
		// GET /api/admin/scraper — list every running scraper.
		// Sorted by name (Manager.List handles ordering).
		Group.GET("", func(e *core.RequestEvent) error {
			return e.JSON(http.StatusOK, Manager.List())
		})

		// POST /api/admin/scraper/start — body {"name":"...","sock":"/path/to/qmp.sock"}.
		// 201 with the started scraper's Info on success. 409 on name collision,
		// 400 on missing fields, 502 when xemu init or game detection fails
		// (most often "unknown title ID 0x...").
		Group.POST("/start", func(e *core.RequestEvent) error {
			var body struct {
				Name string `json:"name"`
				Sock string `json:"sock"`
			}
			if err := e.BindBody(&body); err != nil {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			if body.Name == "" || body.Sock == "" {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "name and sock are required"})
			}

			if err := Manager.Start(body.Name, body.Sock); err != nil {
				if errors.Is(err, scrapermgr.ErrAlreadyRunning) {
					return e.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
				}
				return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
			}

			// Re-read from List so the response carries the live Info (start time,
			// title ID resolved by Detect, etc.) without forcing Manager.Start to
			// return a typed value through the interface.
			for _, info := range Manager.List() {
				if info.Name == body.Name {
					return e.JSON(http.StatusCreated, info)
				}
			}
			// Should never happen — Start succeeded but the name vanished from List.
			return e.NoContent(http.StatusCreated)
		})

		// GET /api/admin/scraper/{name}/inspect — deep-dive view used by the
		// admin debug page. Returns the runner's cached current_state plus the
		// most recent snapshot/tick/events. Fields are nil/empty when the runner
		// has been alive but never observed an in-game tick or snapshot-eligible
		// state transition. 404 when no runner is attached for name.
		Group.GET("/{name}/inspect", func(e *core.RequestEvent) error {
			name := e.Request.PathValue("name")
			if name == "" {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
			}
			st, ok := Manager.Inspect(name)
			if !ok {
				return e.JSON(http.StatusNotFound, map[string]string{"error": "scraper not running"})
			}
			return e.JSON(http.StatusOK, st)
		})

		// POST /api/admin/scraper/stop/{name} — idempotent.
		// Returns 204 whether the runner was found or not (Manager.Stop never
		// errors on unknown names; matches container Stop semantics).
		Group.POST("/stop/{name}", func(e *core.RequestEvent) error {
			name := e.Request.PathValue("name")
			if name == "" {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
			}
			if err := Manager.Stop(name); err != nil {
				return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return e.NoContent(http.StatusNoContent)
		})
	})
}
