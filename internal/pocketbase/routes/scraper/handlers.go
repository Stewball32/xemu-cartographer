package scraper

import (
	"errors"
	"net/http"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/xcclient"
	"github.com/xemu-cartographer/xc-scraper/runner"
)

// Wire returns the daemon control client behind an injected source when the
// scraper feed runs in wire mode (DESIGN-STEP8 D-4): the leaguescraper.Adapter
// exposes Ctl(). The in-process WireAdapter does not, so callers get nil and
// keep today's path. Route packages that only hold a narrower interface
// (PlayControl, MapSource, …) pass that value — the check is structural.
func Wire(src any) *xcclient.Ctl {
	c, ok := src.(interface{ Ctl() *xcclient.Ctl })
	if !ok || c == nil {
		return nil
	}
	return c.Ctl()
}

// UpstreamDown reports whether src is a wire-mode source whose daemon stream
// is currently away. Every proxied call would fail fast with
// xcclient.ErrUpstreamDown, and the bool-returning ports (Status, SetReady,
// AvailableMaps, …) cannot carry that error, so the routes answer 503 up
// front through the same gate the Ctl uses. Always false in-process.
func UpstreamDown(src any) bool {
	ctl := Wire(src)
	return ctl != nil && ctl.Connected != nil && !ctl.Connected()
}

// IsUpstreamDown reports whether a proxied call failed because the daemon
// is away (the Adapter passes xcclient.ErrUpstreamDown through unchanged).
func IsUpstreamDown(err error) bool {
	return errors.Is(err, xcclient.ErrUpstreamDown)
}

// UnavailableMessage is the 503 body every scraper-backed route answers while
// the daemon is away (§16 "503 while daemon down").
const UnavailableMessage = "scraper upstream unavailable"

// Unavailable writes the shared 503 body.
func Unavailable(e *core.RequestEvent) error {
	return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": UnavailableMessage})
}

// PhaseAttaching is the phase POST /start reports in wire mode: the daemon
// accepted the attach (202) and the outcome surfaces in the list phase.
const PhaseAttaching = "attaching"

// startAccepted is the 202 body of POST /start in wire mode.
type startAccepted struct {
	Name  string `json:"name"`
	Sock  string `json:"sock"`
	Phase string `json:"phase"`
}

func init() {
	register(func() {
		// GET /api/admin/scraper — list every running scraper.
		// Sorted by name (Manager.List handles ordering). In wire mode the list
		// is the mirror, which keeps serving until the stale clear (D-12).
		Group.GET("", func(e *core.RequestEvent) error {
			return e.JSON(http.StatusOK, Manager.List())
		})

		// POST /api/admin/scraper/start — body {"name":"...","sock":"/path/to/qmp.sock"}.
		// In-process: 201 with the started runner's Info on success, 502 when
		// xemu QMP init fails (Start blocks in InitWait). 409 on name collision,
		// 400 on missing fields or chokepoint-rejected names (reserved suffix
		// "all", names containing ":" or whitespace — see M5 stage 5b in
		// internal/websocket/rooms/host.go).
		// Wire mode (§16): the daemon answers POST /api/ctl/attach with 202 and
		// runs the attach loop itself, so this route answers 202
		// {"name","sock","phase":"attaching"} and a QMP failure surfaces as the
		// list phase instead of a 502; 503 while the daemon is away.
		// After M5 stage 5a the runner enters Idle and self-detects the
		// running XBE — Start no longer rejects unknown titles, so callers
		// need to poll /api/admin/scraper/{name}/inspect to see whether
		// detection succeeded (phase=ready/live) or the runner is still in
		// Idle awaiting a known title-ID.
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
				if errors.Is(err, runner.ErrAlreadyRunning) {
					return e.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
				}
				if errors.Is(err, runner.ErrInvalidName) {
					return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
				}
				if IsUpstreamDown(err) {
					return Unavailable(e)
				}
				return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
			}

			if Wire(Manager) != nil {
				return e.JSON(http.StatusAccepted, startAccepted{Name: body.Name, Sock: body.Sock, Phase: PhaseAttaching})
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
		// most recent game-data/tick/events. Fields are nil/empty when the runner
		// has been alive but never observed an in-game tick or game-data-eligible
		// state transition. 404 when no runner is attached for name; 503 while
		// the daemon is away in wire mode (Inspect is a read-API proxy there).
		Group.GET("/{name}/inspect", func(e *core.RequestEvent) error {
			name := e.Request.PathValue("name")
			if name == "" {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
			}
			if UpstreamDown(Manager) {
				return Unavailable(e)
			}
			st, ok := Manager.Inspect(name)
			if !ok {
				return e.JSON(http.StatusNotFound, map[string]string{"error": "scraper not running"})
			}
			return e.JSON(http.StatusOK, st)
		})

		// POST /api/admin/scraper/{name}/stop — idempotent.
		// Returns 204 whether the runner was found or not (Manager.Stop never
		// errors on unknown names; matches container Stop semantics). 503 when
		// the daemon is away in wire mode.
		//
		// Path shape is {name}/stop (not the earlier /stop/{name}) so it's
		// consistent with {name}/inspect and {name}/host — and, critically, so
		// POST {name}/stop doesn't collide with POST {name}/host in Go's
		// ServeMux (POST /stop/{name} and POST /{name}/host both match
		// /stop/host, which panics the router at registration). No frontend
		// calls this route (scraper start/stop are discovery/curl-driven); the
		// e2e mocks already expect the {name}/stop form.
		Group.POST("/{name}/stop", func(e *core.RequestEvent) error {
			name := e.Request.PathValue("name")
			if name == "" {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
			}
			if err := Manager.Stop(name); err != nil {
				if IsUpstreamDown(err) {
					return Unavailable(e)
				}
				return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return e.NoContent(http.StatusNoContent)
		})
	})
}
