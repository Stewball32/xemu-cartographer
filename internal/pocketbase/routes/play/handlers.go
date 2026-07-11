package play

import (
	"net/http"

	"github.com/pocketbase/pocketbase/core"

	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/hostrunner"
)

func init() {
	register(registerCurrent)
	register(registerOptions)
	register(registerSelection)
	register(registerReady)
	register(registerUnready)
	register(registerRequest)
	register(registerTeardown)
}

// requireHost guards the endpoints that need the host-runner subsystem. Returns
// false (after writing 503) when it isn't wired.
func requireHost(e *core.RequestEvent) bool {
	if HostRunners == nil {
		_ = e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "host-runner subsystem not enabled"})
		return false
	}
	return true
}

// currentResponse is the play tab's per-poll view of the caller's instance.
// Instance is "" when the caller is idle (not in any live roster).
type currentResponse struct {
	Instance string             `json:"instance"`
	Status   *hostrunner.Status `json:"status"`
}

// GET /api/play/current — the caller's currently-matched container + host-runner
// status (screen, authority, native start counts, selection, ready). Returns 200
// with instance:"" when the caller's gamertag isn't in any live roster (idle),
// mirroring /api/me/match's fail-soft shape.
func registerCurrent() {
	Group.GET("/current", func(e *core.RequestEvent) error {
		if !requireHost(e) {
			return nil
		}
		name, ok := resolveCaller(e)
		if !ok {
			return e.JSON(http.StatusOK, currentResponse{Instance: ""})
		}
		st := HostRunners.Status(name)
		return e.JSON(http.StatusOK, currentResponse{Instance: name, Status: &st})
	})
}

// optionsResponse is the map/gametype picker payload. Maps/Gametypes are READ
// LIVE from the instance (refinement 2) — Available is false when the box can't
// be enumerated yet, and the client must NOT fall back to a fixed list.
type optionsResponse struct {
	Instance         string                   `json:"instance"`
	Available        bool                     `json:"available"`
	Maps             []scraperiface.MapOption `json:"maps"`
	Gametypes        []scraperiface.MapOption `json:"gametypes"`
	SelectedMap      string                   `json:"selected_map"`
	SelectedGametype string                   `json:"selected_gametype"`
}

// GET /api/play/options — the LIVE per-instance map / gametype list + the
// caller's current selection. The list comes from the actual game/disc on that
// box (never a stock table); when the instance can't be enumerated yet it returns
// available:false with empty lists, and the client should let the player enter a
// map name free-form. Idle (no matched instance) returns empty + available:false.
func registerOptions() {
	Group.GET("/options", func(e *core.RequestEvent) error {
		if !requireHost(e) {
			return nil
		}
		resp := optionsResponse{Maps: []scraperiface.MapOption{}, Gametypes: []scraperiface.MapOption{}}
		if name, ok := resolveCaller(e); ok {
			resp.Instance = name
			list := liveMaps(name)
			resp.Available = list.Available
			if len(list.Maps) > 0 {
				resp.Maps = list.Maps
			}
			if len(list.Gametypes) > 0 {
				resp.Gametypes = list.Gametypes
			}
			st := HostRunners.Status(name)
			resp.SelectedMap = st.SelectedMap
			resp.SelectedGametype = st.SelectedGametype
		}
		return e.JSON(http.StatusOK, resp)
	})
}

// POST /api/play/selection — record the player's map / gametype pick. Body
// {"map":"Blood Gulch","gametype":"Team Slayer"}. The chosen names are validated
// against — and their D-pad navigation Steps computed from — the LIVE per-instance
// list. When that list isn't available (not enumerable yet), names are accepted
// free-form with Steps 0 (modded discs make any hardcoded validation wrong).
// Empty field = leave that side unchanged. Returns the updated status.
func registerSelection() {
	Group.POST("/selection", func(e *core.RequestEvent) error {
		if !requireHost(e) {
			return nil
		}
		var body struct {
			Map      string `json:"map"`
			Gametype string `json:"gametype"`
		}
		if err := e.BindBody(&body); err != nil {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		name, ok := resolveCaller(e)
		if !ok {
			return e.JSON(http.StatusNotFound, map[string]string{"error": "no active instance for your gamertag"})
		}
		st := HostRunners.Status(name)
		if !st.Present {
			return e.JSON(http.StatusNotFound, map[string]string{"error": "no host runner attached for " + name})
		}
		if st.Authority != "runner" {
			return e.JSON(http.StatusConflict, map[string]string{
				"error": "instance is not player-controllable right now (" + st.Authority + ")",
			})
		}

		// Preserve the untouched side when only one field is supplied.
		mapName := body.Map
		if mapName == "" {
			mapName = st.SelectedMap
		}
		gametype := body.Gametype
		if gametype == "" {
			gametype = st.SelectedGametype
		}
		if mapName == "" || gametype == "" {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "both map and gametype must be chosen"})
		}

		// Validate against + derive nav Steps from the LIVE list; accept free-form
		// when the instance isn't enumerable yet.
		list := liveMaps(name)
		mapSteps, ok1 := 0, true
		gtSteps, ok2 := 0, true
		if list.Available {
			mapSteps, ok1 = list.IndexOf(list.Maps, mapName)
			gtSteps, ok2 = list.IndexOf(list.Gametypes, gametype)
			if !ok1 {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "map not available on this instance: " + mapName})
			}
			if !ok2 {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "gametype not available on this instance: " + gametype})
			}
		}

		if !HostRunners.SetSelection(name, mapName, mapSteps, gametype, gtSteps) {
			return e.JSON(http.StatusConflict, map[string]string{"error": "selection unavailable for " + name})
		}
		return e.JSON(http.StatusOK, HostRunners.Status(name))
	})
}

// liveMaps returns the per-instance live map list, or an empty (unavailable) list
// when no map source is wired.
func liveMaps(name string) scraperiface.MapList {
	if Maps == nil {
		return scraperiface.MapList{}
	}
	return Maps.AvailableMaps(name)
}

// POST /api/play/ready — the player says "go": arm+start, so the runner presses
// start once the NATIVE preconditions pass (2+ boxes, 2+ teams). Returns status.
func registerReady() {
	Group.POST("/ready", playAction(func(name string) bool {
		return HostRunners.SetReady(name, true)
	}))
}

// POST /api/play/unready — stay armed in the lobby (arm-only). Returns status.
func registerUnready() {
	Group.POST("/unready", playAction(func(name string) bool {
		return HostRunners.SetReady(name, false)
	}))
}

// POST /api/play/request — claim a fresh hosting session on the caller's matched
// box: reset the start request so the runner sets up an arm-only lobby (the
// runner auto-navigates to a host lobby whenever it's runner-driven). Returns
// status. Container provisioning itself stays admin-only (routes/containers).
func registerRequest() {
	Group.POST("/request", playAction(func(name string) bool {
		return HostRunners.SetReady(name, false)
	}))
}

// POST /api/play/teardown — the player ends their session: un-ready AND clear the
// map/gametype selection so the runner re-parks at map-select awaiting the next
// player's pick. Does NOT change arbitration authority (admin-only) or stop the
// container. Returns status.
func registerTeardown() {
	Group.POST("/teardown", playAction(func(name string) bool {
		HostRunners.SetReady(name, false)
		return HostRunners.ClearSelection(name)
	}))
}

// playAction wires the resolve → authority-guard → apply → status shape shared by
// the control POSTs. Player actions only touch player-intent (ready / selection),
// NEVER arbitration authority — a player can't wrest control from an admin. If the
// runner isn't runner-driven (an admin took over, or hosting is disabled), the
// action is refused with 409 so admin control always wins. apply returns false
// when no runner is attached (→ 404).
func playAction(apply func(name string) bool) func(e *core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if !requireHost(e) {
			return nil
		}
		name, ok := resolveCaller(e)
		if !ok {
			return e.JSON(http.StatusNotFound, map[string]string{"error": "no active instance for your gamertag"})
		}
		st := HostRunners.Status(name)
		if !st.Present {
			return e.JSON(http.StatusNotFound, map[string]string{"error": "no host runner attached for " + name})
		}
		if st.Authority != "runner" {
			return e.JSON(http.StatusConflict, map[string]string{
				"error": "instance is not player-controllable right now (" + st.Authority + ")",
			})
		}
		if !apply(name) {
			return e.JSON(http.StatusNotFound, map[string]string{"error": "no host runner attached for " + name})
		}
		return e.JSON(http.StatusOK, HostRunners.Status(name))
	}
}
