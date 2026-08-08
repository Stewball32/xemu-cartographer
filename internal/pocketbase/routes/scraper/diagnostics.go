package scraper

import (
	"net/http"

	"github.com/pocketbase/pocketbase/core"

	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/hostrunner"
)

// MapSource supplies the LIVE-enumerated map/gametype carousel for a box (the same
// list the /play picker uses). Injected from cmd/server/main.go (the scraper
// manager). An interface so this package keeps no compile-time dependency on the
// manager; nil until wired (the endpoint then omits the enumerated lists).
type MapSource interface {
	AvailableMaps(name string) scraperiface.MapList
}

var Maps MapSource

// SetMapSource wires the live map source. Call before RegisterAll.
func SetMapSource(m MapSource) { Maps = m }

// diagnosticsResponse is the admin diagnostics-panel payload: the live per-tick
// scraper reads plus the enumerated map/gametype NAMES (so the panel shows the
// selected pick against the full carousel).
type diagnosticsResponse struct {
	hostrunner.Diagnostics
	EnumeratedMaps      []string `json:"enumerated_maps"`
	EnumeratedGametypes []string `json:"enumerated_gametypes"`
}

func optionNames(opts []scraperiface.MapOption) []string {
	out := make([]string, len(opts))
	for i, o := range opts {
		out[i] = o.Name
	}
	return out
}

func init() {
	register(func() {
		// GET /api/admin/scraper/{name}/diagnostics — the LIVE scraper-read snapshot
		// (dela path, resolved MenuItem + screen classification, map/gametype cursors,
		// game_connection, pregame sentinel) + enumerated + selected map/gametype
		// names. Admin-gated (the group binds RequireAuth + RequireAdmin). Feeds the
		// admin kiosk diagnostics panel so an operator watches the box AND its live
		// reads side-by-side, reporting a dela/menu_item fingerprint for any screen
		// without grepping beta.log.
		Group.GET("/{name}/diagnostics", func(e *core.RequestEvent) error {
			if HostRunners == nil {
				return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "host-runner subsystem not enabled"})
			}
			name := e.Request.PathValue("name")
			if name == "" {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
			}
			resp := diagnosticsResponse{Diagnostics: HostRunners.Diagnostics(name)}
			if Maps != nil {
				ml := Maps.AvailableMaps(name)
				resp.EnumeratedMaps = optionNames(ml.Maps)
				resp.EnumeratedGametypes = optionNames(ml.Gametypes)
			}
			return e.JSON(http.StatusOK, resp)
		})
	})
}
