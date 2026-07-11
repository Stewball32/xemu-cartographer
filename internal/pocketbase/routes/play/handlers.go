package play

import (
	"net/http"

	"github.com/pocketbase/pocketbase/core"

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

// optionsResponse is the map/gametype picker payload.
type optionsResponse struct {
	Instance         string   `json:"instance"`
	Maps             []string `json:"maps"`
	Gametypes        []string `json:"gametypes"`
	SelectedMap      string   `json:"selected_map"`
	SelectedGametype string   `json:"selected_gametype"`
}

// GET /api/play/options — the map / gametype catalog + the caller's current
// selection. The catalog is static (CE stock maps/gametypes); the selection is
// the runner's recorded intent.
func registerOptions() {
	Group.GET("/options", func(e *core.RequestEvent) error {
		if !requireHost(e) {
			return nil
		}
		cat := hostrunner.DefaultCatalog()
		resp := optionsResponse{Maps: cat.Maps, Gametypes: cat.Gametypes}
		// The catalog renders even when idle; selection needs a matched instance.
		if name, ok := resolveCaller(e); ok {
			st := HostRunners.Status(name)
			resp.Instance = name
			resp.SelectedMap = st.SelectedMap
			resp.SelectedGametype = st.SelectedGametype
		}
		return e.JSON(http.StatusOK, resp)
	})
}

// POST /api/play/selection — record the player's map / gametype pick. Body
// {"map":"Blood Gulch","gametype":"Team Slayer"}. Values are validated against
// the catalog (empty = leave as-is / default). Returns the updated status.
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
		if body.Map != "" && !inCatalog(hostrunner.CEMaps, body.Map) {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "unknown map: " + body.Map})
		}
		if body.Gametype != "" && !inCatalog(hostrunner.CEGametypes, body.Gametype) {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "unknown gametype: " + body.Gametype})
		}
		name, ok := resolveCaller(e)
		if !ok {
			return e.JSON(http.StatusNotFound, map[string]string{"error": "no active instance for your gamertag"})
		}
		// Preserve the untouched side when only one field is supplied.
		st := HostRunners.Status(name)
		if !st.Present {
			return e.JSON(http.StatusNotFound, map[string]string{"error": "no host runner attached for " + name})
		}
		if st.Authority != "runner" {
			return e.JSON(http.StatusConflict, map[string]string{
				"error": "instance is not player-controllable right now (" + st.Authority + ")",
			})
		}
		mapName := body.Map
		if mapName == "" {
			mapName = st.SelectedMap
		}
		gametype := body.Gametype
		if gametype == "" {
			gametype = st.SelectedGametype
		}
		if !HostRunners.SetSelection(name, mapName, gametype) {
			return e.JSON(http.StatusConflict, map[string]string{"error": "selection unavailable for " + name})
		}
		return e.JSON(http.StatusOK, HostRunners.Status(name))
	})
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

// POST /api/play/teardown — the player ends their session: clear the start
// request so the box stops trying to start a game. Does NOT change arbitration
// authority (admin-only) or stop the container. Returns status.
func registerTeardown() {
	Group.POST("/teardown", playAction(func(name string) bool {
		return HostRunners.SetReady(name, false)
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

func inCatalog(catalog []string, v string) bool {
	for _, c := range catalog {
		if c == v {
			return true
		}
	}
	return false
}
