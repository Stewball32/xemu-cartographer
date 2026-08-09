package routes

import (
	"net/http"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/overlaytoken"
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
func v2Roster(players []sc.GamePlayer) []map[string]any {
	out := make([]map[string]any, 0, len(players))
	for _, p := range players {
		out = append(out, map[string]any{
			"index": p.Index, "name": p.Name, "team": p.Team, "armor_color": p.ArmorColor,
			"score": p.Score, "kills": p.Kills, "deaths": p.Deaths, "assists": p.Assists,
			"team_kills": p.TeamKills, "suicides": p.Suicides,
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

// filterDummies drops the neutral-host dummy + globally-allowlisted dummy
// gamertags from a roster before it reaches any overlay, via the shared M10d
// roster.FilterRoster. The reliable signature is a CONFIG one: the host's own
// local player is dropped ONLY when its container is flagged is_neutral_host —
// so a host that actually plays (not flagged neutral) keeps its player. Plus
// the dummy_gamertags name allowlist. Best-effort: DB read errors → unfiltered.
//
// dummyFilter returns BOTH the ready-to-apply roster.Config AND the raw dummy
// gamertag strings. The raw list is echoed to the client so the WS-push console
// path (which reads the UNFILTERED host:<instance> broadcast, not this route's
// pre-filtered snapshot) can re-apply the same filter client-side. One DB read
// for both.
func dummyFilter(app core.App, instance string) (roster.Config, []string) {
	neutral := false
	if rec, err := app.FindFirstRecordByFilter("containers", "name = {:n}", dbx.Params{"n": instance}); err == nil && rec != nil {
		neutral = rec.GetBool("is_neutral_host")
	}
	var raw []string
	if rows, err := app.FindAllRecords("dummy_gamertags"); err == nil {
		raw = make([]string, 0, len(rows))
		for _, r := range rows {
			raw = append(raw, r.GetString("gamertag"))
		}
	}
	return roster.Config{IsNeutralHost: neutral, DummyGamertags: roster.BuildDummySet(raw)}, raw
}

// mintConsoleToken best-effort mints a read-only overlay token scoped to
// host:<instance> so the ?console= overlay can ride the EXISTING per-instance WS
// rooms (no Hub change, no new room type). Returns "" on any failure, in which
// case the client falls back to the HTTP poll path. Auth is deferred per the PoC
// — this endpoint is unauthenticated, so the token is handed out freely for now.
// Minting is throttled client-side (once at start + on genuine migration), so
// this does not spawn a registry row per poll.
func mintConsoleToken(app core.App, instance string) string {
	m, err := overlaytoken.Mint(app, "host:"+instance, "console-overlay:"+instance, 0, nil, time.Now())
	if err != nil {
		app.Logger().Warn("overlay console: token mint failed (client will poll)", "instance", instance, "err", err)
		return ""
	}
	return m.Token
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
		// One DB read: the filter config (applied to the snapshot below) + the raw
		// dummy list (echoed so the WS-push path can re-filter client-side).
		cfg, rawDummies := dummyFilter(e.App, instance)
		// engine_tick (0x0C, free-running) kept alongside game_elapsed_ticks
		// (0x10, match-elapsed) so the scorebug can render the count-up clock and
		// both remain comparable on a live match.
		game := map[string]any{"phase": st.Phase, "engine_tick": st.Tick}
		scenario := map[string]any{}
		if gd != nil {
			game["game_elapsed_ticks"] = gd.ElapsedTicks
			game["config"] = map[string]any{
				"gametype": gd.Gametype, "is_team_game": gd.IsTeamGame, "score_limit": gd.ScoreLimit,
			}
			game["team_scores"] = gd.TeamScores
			game["players"] = v2Roster(roster.FilterRoster(gd.Players, cfg))
			game["machines"] = gd.Machines
			scenario["map"] = gd.Map
		}
		return e.JSON(http.StatusOK, map[string]any{
			"console":       name,
			"instance":      instance,
			"machine_index": machineIndex,
			"machine_name":  machineName,
			// WS-push path: a read-only token scoped to host:<instance> (empty if
			// mint failed → client polls) + the token's room, so the client can
			// reuse the existing per-instance rooms via scraperWSV2.
			"token":      mintConsoleToken(e.App, instance),
			"token_room": "host:" + instance,
			// Dummy-filter config for the client to re-apply on the unfiltered WS
			// broadcast (the snapshot's game.players above is already filtered).
			"filter":   map[string]any{"is_neutral_host": cfg.IsNeutralHost, "dummy_gamertags": rawDummies},
			"game":     game,
			"tick":     map[string]any{"players": v2Tick(st.LatestTick)},
			"scenario": scenario,
		})
	})
}
