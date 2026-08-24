package routes

import (
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/core"

	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
	scraperroutes "github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/scraper"
	sc "github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/roster"
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
func init() {
	register(registerOverlayConsole)
	register(registerOverlayConsoleList)
}

// consoleEntry is one selectable console for the Studio picker: its name, which
// host currently sees it, whether it's a local/host console, and its lobby
// machine index (-1 = the host's own console with no live lobby).
type consoleEntry struct {
	Console      string `json:"console"`
	Instance     string `json:"instance"`
	IsLocal      bool   `json:"is_local"`
	MachineIndex int    `json:"machine_index"`
}

// listConsoles aggregates every console name currently visible across all
// running hosts — each host's own console (xbox_name) plus every System Link
// lobby peer (game_data.machines) — deduped by name (a lobby-machine entry,
// which carries a machine index, wins over the bare xbox_name). This is the
// console index the overlays resolve against; Studio lists it so the operator
// targets by name.
func listConsoles(mgr scraperiface.Inspect) []consoleEntry {
	seen := map[string]consoleEntry{}
	order := []string{}
	add := func(e consoleEntry) {
		k := ovSanitize(e.Console)
		if k == "" {
			return
		}
		if existing, ok := seen[k]; ok {
			if existing.MachineIndex < 0 && e.MachineIndex >= 0 {
				seen[k] = e // prefer the lobby-machine entry (has an index)
			}
			return
		}
		seen[k] = e
		order = append(order, k)
	}
	for _, info := range mgr.List() {
		st, ok := mgr.Inspect(info.Name)
		if !ok {
			continue
		}
		if info.XboxName != "" {
			add(consoleEntry{Console: info.XboxName, Instance: info.Name, IsLocal: true, MachineIndex: -1})
		}
		if st.GameData != nil {
			for _, m := range st.GameData.Machines {
				add(consoleEntry{
					Console:      m.Name,
					Instance:     info.Name,
					IsLocal:      m.IsLocal != nil && *m.IsLocal,
					MachineIndex: m.Index,
				})
			}
		}
	}
	out := make([]consoleEntry, 0, len(order))
	for _, k := range order {
		out = append(out, seen[k])
	}
	return out
}

func registerOverlayConsoleList(se *core.ServeEvent) {
	// PUBLIC PoC (security deferred): every console name currently visible, for
	// the Studio picker. See registerOverlayConsole for the auth caveat.
	se.Router.GET("/api/overlay/consoles", func(e *core.RequestEvent) error {
		mgr := scraperroutes.Manager
		if mgr == nil {
			return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "scraper not running"})
		}
		return e.JSON(http.StatusOK, map[string]any{"consoles": listConsoles(mgr)})
	})
}

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
func v2Roster(players []sc.GamePlayer, accum map[int]sc.PlayerAccum) []map[string]any {
	out := make([]map[string]any, 0, len(players))
	for _, p := range players {
		acc := accum[p.Index]
		out = append(out, map[string]any{
			"index": p.Index, "name": p.Name, "team": p.Team, "armor_color": p.ArmorColor,
			"score": p.Score, "kills": p.Kills, "deaths": p.Deaths, "assists": p.Assists,
			"team_kills": p.TeamKills, "suicides": p.Suicides,
			"kill_streak": p.KillStreak, "shots_fired": p.ShotsFired, "shots_hit": p.ShotsHit,
			"is_local": p.IsLocal, "local_index": p.LocalIndex, "machine_index": p.MachineIndex,
			// Accumulated match stats (same names as the WS game class).
			"acc_shots_fired": acc.ShotsFired, "acc_grenade_throws": acc.GrenadeThrows,
			"acc_melees": acc.Melees, "acc_damage_dealt": acc.DamageDealt,
			"acc_damage_received": acc.DamageReceived, "acc_camo_pickups": acc.CamoPickups,
			"acc_overshield_pickups": acc.OvershieldPickups, "best_kill_streak": acc.BestKillStreak,
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

// activeFromAccum projects an accumulator snapshot into the filter's
// index→latched-Active map (mirror of the manager's activeLocals — kept local
// to avoid exporting a one-liner across the scraperiface boundary).
func activeFromAccum(snap map[int]sc.PlayerAccum) map[int]bool {
	if len(snap) == 0 {
		return nil
	}
	out := make(map[int]bool, len(snap))
	for idx, st := range snap {
		if st.Active {
			out[idx] = true
		}
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
		// Snapshot roster is filtered server-side with the SAME unified rule as
		// the scraper's game_filtered broadcast: local seats hidden until the
		// accumulator latches them Active; is_neutral_host = hard override;
		// dummy_gamertags allowlist. This snapshot only serves the HTTP-poll
		// fallback; the WS-push path gets the game_filtered class.
		cfg := roster.LoadConfig(e.App, instance)
		cfg.HideInactiveLocals = true
		cfg.ActiveLocals = activeFromAccum(st.PlayerAccum)
		// engine_tick (0x0C) is the MATCH CLOCK — starts ~0 at match start and
		// counts up at 30Hz (live-verified 2026-08-08); the scorebug renders it.
		// game_elapsed_ticks (0x10) is a DEAD VALUE (stuck at ~1), kept on the
		// wire only for payload-shape compatibility.
		game := map[string]any{"phase": st.Phase, "engine_tick": st.Tick}
		scenario := map[string]any{}
		if gd != nil {
			game["game_elapsed_ticks"] = gd.ElapsedTicks
			game["config"] = map[string]any{
				"gametype": gd.Gametype, "is_team_game": gd.IsTeamGame, "score_limit": gd.ScoreLimit,
			}
			game["team_scores"] = gd.TeamScores
			game["players"] = v2Roster(roster.FilterRoster(gd.Players, cfg), st.PlayerAccum)
			game["machines"] = gd.Machines
			scenario["map"] = gd.Map
		}
		// The client uses this purely to resolve console→instance, then opens a
		// tokenless WS (?console=NAME) and subscribes to host:<instance>:
		// game_filtered/tick/scenario. No token, no filter config — the WS door is
		// tokenless and filtering is server-side (game_filtered class).
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
