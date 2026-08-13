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

// ReadoutSource supplies the instance's CURRENT per-tick scraper readout — the same
// one the host runner ticks on and navfp logs from. Injected from cmd/server/main.go
// (the scraper manager). Without it the panel would fall back to the host Registry's
// last event, which only exists while a runner is attached and ticking — that is what
// froze the panel at tick 0 on an observed-only box.
type ReadoutSource interface {
	Readout(name string) (hostrunner.ScraperReadout, bool)
}

var Readouts ReadoutSource

// SetReadoutSource wires the live per-tick readout source. Call before RegisterAll.
func SetReadoutSource(r ReadoutSource) { Readouts = r }

// diagnosticsResponse is the admin diagnostics-panel payload: the live per-tick
// scraper reads plus the enumerated map/gametype NAMES and the HIGHLIGHTED (live
// carousel-selection) names resolved from the cursor index — so the panel can show
// the three DISTINCT parameters Stewart called out: what's HIGHLIGHTED (the live
// selection being driven), what's PICKED (the player's target, Diagnostics.Selected*),
// and what's LOADED (Diagnostics.Map/Gametype — read-only, updated only when A commits).
type diagnosticsResponse struct {
	hostrunner.Diagnostics
	HighlightedMap      string   `json:"highlighted_map"`      // enumerated name at the live map cursor index
	HighlightedGametype string   `json:"highlighted_gametype"` // enumerated name at the live gametype cursor index
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

// nameAtCursor resolves the WIDGET-space carousel index to an enumerated name.
// Maps have no prefix (widget index == enumeration index). Gametypes PREPEND the
// user's custom variants ahead of the built-ins, so the enumeration (built-in) list
// starts at prefix = liveCount − len(names); a cursor sitting in that prefix region
// is a custom variant not in this list. "" when unresolvable.
func nameAtCursor(names []string, cursorIndex, cursorCount int) string {
	prefix := cursorCount - len(names)
	if prefix < 0 {
		prefix = 0
	}
	if ai := cursorIndex - prefix; ai >= 0 && ai < len(names) {
		return names[ai]
	}
	if cursorIndex >= 0 && cursorIndex < prefix {
		return "(custom variant)"
	}
	return ""
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
			// Overlay the CURRENT tick's reads so the panel is live even when no host
			// runner is attached (registry events only flow while a runner ticks).
			if Readouts != nil {
				if ro, ok := Readouts.Readout(name); ok {
					resp.Diagnostics.ApplyReadout(ro)
					resp.Present = true
				}
			}
			if Maps != nil {
				ml := Maps.AvailableMaps(name)
				resp.EnumeratedMaps = optionNames(ml.Maps)
				resp.EnumeratedGametypes = optionNames(ml.Gametypes)
				// The HIGHLIGHTED (live carousel-selection) name — distinct from the
				// LOADED map (resp.Map) and the player's PICK (resp.SelectedMap).
				resp.HighlightedMap = nameAtCursor(resp.EnumeratedMaps, resp.MapCursor.Index, resp.MapCursor.Count)
				resp.HighlightedGametype = nameAtCursor(resp.EnumeratedGametypes, resp.GametypeCursor.Index, resp.GametypeCursor.Count)
			}
			return e.JSON(http.StatusOK, resp)
		})
	})
}
