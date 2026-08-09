package routes

import (
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/core"

	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
	scraperroutes "github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/scraper"
	sc "github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// PROOF OF CONCEPT — target an overlay purely by CONSOLE NAME (no instance /
// container, no token). Given a console name, this finds which running host box
// currently sees that console — either as its OWN console (xbox_name) or as a
// peer in its System Link lobby (game_data.machines) — and returns that host's
// live snapshot plus which machine matched. An OBS overlay polls this by console
// name and RE-RESOLVES every tick, so it survives the container being recreated
// (the console name is the stable constant; the instance id churns).
//
// Built on the M09 identity mechanic (the host already scrapes peer console
// names into game_data.machines — see internal/scraper/manager/membership.go).
//
// ⚠️ SECURITY DEFERRED: this endpoint is UNAUTHENTICATED for the PoC. Before
// production, gate it (console-scoped overlay token / LAN allowlist) — tracked
// in the overlay reboot plan.
func init() { register(registerOverlayConsole) }

func ovSanitize(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// findMachineByName returns the index of the lobby machine whose name matches
// want (case-insensitive) — this is the console's per-console filter target.
func findMachineByName(machines []sc.GameMachine, want string) (idx int, name string, ok bool) {
	w := ovSanitize(want)
	for _, m := range machines {
		if ovSanitize(m.Name) == w {
			return m.Index, m.Name, true
		}
	}
	return 0, "", false
}

// resolveConsole locates the instance currently hosting a console name. A live
// lobby-machine match wins (it yields a machine filter); otherwise an instance
// whose own xbox_name matches is the fallback (machine_index -1 = the whole
// instance / its own local players).
func resolveConsole(mgr scraperiface.Inspect, console string) (instance string, machineIndex int, machineName string, st scraperiface.InspectState, ok bool) {
	want := ovSanitize(console)
	var fbInstance, fbName string
	var fbState scraperiface.InspectState
	fbFound := false
	for _, info := range mgr.List() {
		state, present := mgr.Inspect(info.Name)
		if !present {
			continue
		}
		if state.GameData != nil {
			if i, n, found := findMachineByName(state.GameData.Machines, want); found {
				return info.Name, i, n, state, true
			}
		}
		if !fbFound && ovSanitize(info.XboxName) == want {
			fbInstance, fbName, fbState, fbFound = info.Name, info.XboxName, state, true
		}
	}
	if fbFound {
		return fbInstance, -1, fbName, fbState, true
	}
	return "", 0, "", scraperiface.InspectState{}, false
}

// v2Roster reshapes the reader GamePlayer roster into the v2 GameRosterPlayer
// JSON the overlay client already parses (field names match; this is a pass-
// through of the overlay-relevant subset).
func v2Roster(players []sc.GamePlayer) []map[string]any {
	out := make([]map[string]any, 0, len(players))
	for _, p := range players {
		out = append(out, map[string]any{
			"index": p.Index, "name": p.Name, "team": p.Team, "armor_color": p.ArmorColor,
			"score": p.Score, "kills": p.Kills, "deaths": p.Deaths, "assists": p.Assists,
			"kill_streak": p.KillStreak, "shots_fired": p.ShotsFired, "shots_hit": p.ShotsHit,
			"is_local": p.IsLocal, "local_index": p.LocalIndex, "machine_index": p.MachineIndex,
		})
	}
	return out
}

// v2Tick reshapes the reader tick roster into the slim per-index live-state the
// overlay client reads (alive / health / shields / camo / respawn).
func v2Tick(t *sc.TickPayload) []map[string]any {
	if t == nil {
		return nil
	}
	out := make([]map[string]any, 0, len(t.Players))
	for _, p := range t.Players {
		out = append(out, map[string]any{
			"index": p.Index, "alive": p.Alive, "health": p.Health, "shields": p.Shields,
			"has_camo": p.HasCamo, "respawn_in_ticks": p.RespawnInTicks,
		})
	}
	return out
}

func registerOverlayConsole(se *core.ServeEvent) {
	se.Router.GET("/api/overlay/console/{name}", func(e *core.RequestEvent) error {
		mgr := scraperroutes.Manager
		if mgr == nil {
			return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "scraper not running"})
		}
		name := e.Request.PathValue("name")
		if strings.TrimSpace(name) == "" {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "console name required"})
		}
		instance, machineIndex, machineName, st, ok := resolveConsole(mgr, name)
		if !ok {
			return e.JSON(http.StatusNotFound, map[string]any{"error": "console not found in any live lobby", "console": name})
		}
		gd := st.GameData
		game := map[string]any{"phase": st.Phase}
		scenario := map[string]any{}
		if gd != nil {
			game["config"] = map[string]any{
				"gametype": gd.Gametype, "is_team_game": gd.IsTeamGame, "score_limit": gd.ScoreLimit,
			}
			game["team_scores"] = gd.TeamScores
			game["players"] = v2Roster(gd.Players)
			game["machines"] = gd.Machines
			scenario["map"] = gd.Map
		}
		return e.JSON(http.StatusOK, map[string]any{
			"console":       name,
			"instance":      instance,
			"machine_index": machineIndex,
			"machine_name":  machineName,
			"game":          game,
			"tick":          map[string]any{"players": v2Tick(st.LatestTick)},
			"scenario":      scenario,
		})
	})
}
