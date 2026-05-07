# Roadmap

Migration plan for xemu-cartographer: a real-time game-state scraper for Xbox titles running in [xemu](https://xemu.app/), rebuilt on top of a clean Go + PocketBase + Disgo + SvelteKit template.

Prior implementation is preserved at [atlas/xemu-cartographer-legacy/](atlas/xemu-cartographer-legacy/). HaloCaster (the older Halo-specific Python/C# sibling) is at [atlas/HaloCaster/](atlas/HaloCaster/) and holds the richest set of Halo: CE memory offsets. Everything under `atlas/` is **reference-only and must be re-verified before porting** — offsets, patterns, and APIs may have drifted or been wrong to begin with.

Milestones, not dates. Generally each blocks the next, though M3 was ported early (out of sequence) to provide a test substrate for M1+M2 — see [M3 status](#milestone-3--container-lifecycle-podman).

---

## Milestone 0 — Template cleanup

Bring the fresh template to a clean starting point.

- [x] Rename `stew-site-template` / `github.com/youruser/yourproject` → `xemu-cartographer` / `github.com/Stewball32/xemu-cartographer`.
- [x] Document `atlas/` contents for future Claude sessions.
- [x] **Follow-up turn** — strip template demo content:
  - Delete `sveltekit/src/routes/examples/`.
  - Drop the `posts` collection + hooks.
  - Keep the placeholder `ping` Discord command for now
  - Keep OAuth providers for now since they generate dynamically.
  - Reduce seed data to superuser-only.

## Milestone 1 — xemu memory bridge

Foundation. Gets the server able to read memory from any xemu-running Xbox game.

**Status:** Ported. [internal/xemu/](internal/xemu/) (mem.go, qmp.go, instance.go) and [internal/scraper/](internal/scraper/) (scraper.go, types.go, state.go) match the legacy implementation. Smoke-tested against a native xemu install via `GET /api/admin/xemu/probe?sock=<path>` ([internal/pocketbase/routes/xemu/probe.go](internal/pocketbase/routes/xemu/probe.go)): PID discovery + QMP handshake + base HVA + low-GVA translation + `/proc/<pid>/mem` reads all confirmed working — XBE magic at GVA `0x00010000` reads back as `0x48454258` ("XBEH"), title ID round-trips out of the certificate. Empty registry returns `detect: unknown title ID 0x...` as expected.

Small extensions on top of the legacy port:

- `findPID` now matches a bare `xemu*` binary in addition to `AppRun`, so native installs work alongside the containerised AppImage path.
- `Instance.PID` field and `Mem.Base()` accessor surfaced for diagnostics (used by the probe route).
- [internal/pocketbase/routes/middleware/admin.go](internal/pocketbase/routes/middleware/admin.go) `RequireAdmin()` admits PocketBase superusers in addition to `users.isAdmin=true` records — aligns the middleware with what CLAUDE.md already documented.

Native-xemu host gotcha to remember for M2 dev work: xemu is typically installed with `CAP_NET_ADMIN | CAP_NET_RAW` file caps for pcap netplay, which makes the process non-dumpable and flips `/proc/<pid>/*` ownership to `root:root` — `/proc/<pid>/mem` becomes unreadable to the same UID even with `kernel.yama.ptrace_scope=0`. Workarounds: `sudo setcap -r $(which xemu)` (drops netplay caps), or grant the server binary `CAP_SYS_PTRACE` (bypasses both Yama and the dumpable check). M3's containerised deployment runs the server rooted inside the container PID namespace and side-steps both.

## Milestone 2 — Halo: CE scraper

**Status:** Ported. End-to-end smoke-tested against a native xemu running Halo: CE in splitscreen — `POST /api/admin/scraper/start` auto-detects title `0x4D530004`, runner streams snapshot/tick/event envelopes at exactly 30Hz to WebSocket clients in the new `overlay` room, both local players' positions / aim vectors / health+shields / weapons (incl. ammo, energy charge, energy-vs-ballistic flag, full tag names like `weapons\sniper rifle\sniper rifle`) / camo + overshield bools all render correctly. Stop is idempotent (POST `/stop/{name}` → 204). One Yama gotcha to remember: `kernel.yama.ptrace_scope=0` (or `setcap cap_sys_ptrace=eip` on the dev binary) is required for the server to read `/proc/<xemu-pid>/mem`. The legacy file-cap workaround documented in M1 still applies if your xemu install carries `CAP_NET_*` for pcap netplay.

What landed:

- **2a — full offset audit.** Reconciled all 515 hex constants from `atlas/HaloCaster/HaloCE/halocaster.py` against the 128-offset legacy Go table. Active read-path constants live in [internal/scraper/haloce/offsets.go](internal/scraper/haloce/offsets.go); every other corroborated offset organised by struct in [internal/scraper/haloce/offsets_reference.go](internal/scraper/haloce/offsets_reference.go). Each constant carries a `// halocaster.py:NNN` origin tag. All marked `unverified` until M19's runtime sanity-check pass.
- **2b — scraper code ported.** [reader.go](internal/scraper/haloce/reader.go), [events.go](internal/scraper/haloce/events.go) (19 event types via stat-diff + damage-table fallback), [game.go](internal/scraper/haloce/game.go) (`init()` registers Halo: CE with `scraper.Lookup`), [xboxname.go](internal/scraper/haloce/xboxname.go).
- **2c — WS wiring.** New [internal/scraper/manager](internal/scraper/manager) package owns per-instance lifecycle (Start / Stop / List) and the 30Hz tick goroutine. Decision: **wrap, not extend** — every broadcast becomes `Message{Type:"scraper", Room:"overlay", Payload:<envelope-json>}` so the wire schema stays uniform across all rooms ([loop.go](internal/scraper/manager/loop.go)). New `overlay` room with `RequireAuth` ([rooms/overlay.go](internal/websocket/rooms/overlay.go)). New `Scraper` field on `guards.Services` backed by `internal/guards/interfaces/scraper/` (one-method-per-file).
- **2d — admin routes + main.go wiring.** `GET /api/admin/scraper`, `POST /api/admin/scraper/start`, `POST /api/admin/scraper/stop/{name}` ([routes/scraper](internal/pocketbase/routes/scraper)), all gated by `RequireAuth + RequireAdmin`. `cmd/server/main.go` builds the `Services` skeleton early so the scraper manager gets a stable `*Services` pointer; subsystems mutate fields as they come up. Blank import `_ "internal/scraper/haloce"` triggers the title-ID registration.

### M2 follow-ups (deferred)

- ~~**Snapshot replay for late joiners.**~~ Resolved during M4 with option (a): each `runner` caches the most-recent `websocket.Message` bytes for its snapshot envelope; the `join_room` handler replays them via the new `SendRaw` event capability when a client subscribes to `overlay`. See `internal/scraper/manager/{runner.go,loop.go,manager.go}`, `internal/guards/interfaces/scraper/snapshot.go`, `internal/websocket/handlers/join_room.go`.
- **Investigate `power_items: null` in tick payloads.** During the smoke test the initial snapshot's `PowerItemSpawns` came back empty (likely the scenario wasn't fully loaded when the scraper started, since power-item resolution depends on world-object scanning). Worth re-running the smoke test with start-after-match-ready and confirming spawns populate; if they still don't, that's a Halo offset divergence to chase during M19.

### 2a. Offset audit (prerequisite)

The legacy Go offset table has 128 hex constants; HaloCaster's `HaloCE/halocaster.py` has 515 scattered across 2587 lines. Before trusting the legacy table as complete:

1. Read `atlas/HaloCaster/HaloCE/halocaster.py` end-to-end, extracting every memory-offset-like constant with surrounding context (what struct, what field, what read type).
2. Diff the extracted set against `atlas/xemu-cartographer-legacy/internal/scraper/haloce/offsets.go`.
3. Categorize the deltas:
   - Genuinely missing offsets the legacy reader never used → port them.
   - Non-offsets (struct sizes, magic values, indexing math) → document in comments, don't port.
   - Offsets that exist in both but differ in value → investigate (xemu vs. real-Xbox divergence is plausible).
4. Produce a reconciled `internal/scraper/haloce/offsets.go` in the new repo, each offset annotated with its HaloCaster origin (file + line) and verification status.
5. Flag offsets needing runtime verification for Milestone 19's sanity-check work.

### 2b. Port the scraper code

- Port `internal/scraper/haloce/{reader.go,events.go,game.go}` using the reconciled offset table.
- If the audit surfaced offsets for fields the legacy reader never consumed, extend `reader.go` to populate them.

### 2c. Wire to the template's WebSocket Hub

Adapt the legacy tick-loop to the template's `internal/websocket/` Hub. Decide during this milestone:

- **(a)** Wrap the legacy `Envelope{Type, Instance, Tick, Payload}` inside the template's existing `message.Message`, or
- **(b)** Extend the template's `message.Message` to carry the legacy envelope directly.

### 2d. Smoke test

Halo: CE match in manually-started xemu → snapshots + ticks + events flow to `/api/ws` clients. Fields added from the audit render plausibly in a debug overlay.

## Milestone 3 — Container lifecycle (Podman)

This is load-bearing — the product has no real UX without it.

**Status:** Ported early. `internal/podman/`, `internal/discovery/`, the six `/api/admin/containers/*` HTTP handlers, env-driven config, and the `CONTAINERS_PODMAN_CMD` rooted-podman escalation are all in. End-to-end create + start + stop + delete + QMP-socket discovery has been smoke-tested against real containers. Two items remain (see follow-ups below): the discovery → scraper auto-start callback (depends on M1+M2) and the `jlesage/firefox` kiosk container's X11 init issue.

- Copy `containers/xemu/init/{01-setup-toml.sh,02-patch-toml.sh,03-setup-hdd.sh,.env}` verbatim into the new repo's `containers/xemu/init/`.
- Port `internal/podman/{podman.go,ports.go,state.go,ports_test.go}` as-is (clean, no known bugs).
- Port `internal/discovery/` socket-directory watcher; wire it to the scraper registry so new `.sock` files in the shared QMP dir auto-start a scraper.
- Port the 6 `/api/containers/*` HTTP handlers from legacy `cmd/cartographer/main.go` into a new `internal/pocketbase/routes/containers.go`. Adapt to PocketBase's `ServeMux` and add the template's auth middleware (legacy assumed localhost-only).
- Extend `xemu-cartographer.toml.example` or fold container config into the root `.env` / a new `config.toml`; decide during porting.
- **Smoke test:** POST `/api/containers` creates an instance → POST `/start` boots xemu + browser containers → scraper auto-connects → live data flows → POST `/stop` + DELETE tears down cleanly.

### M3 follow-ups (deferred)

- ~~**Browser kiosk Firefox crashes inside `jlesage/firefox` container.**~~ Resolved — root cause was the host's OCI runtime, not the image. With `runc` 1.4.x as podman's runtime, jlesage's Xvnc rejects every X client with `Authorization required, but no authorization protocol specified` and Firefox + xcompmgr never connect; with `crun` the same image bits work cleanly. [.env.example](.env.example) now defaults `CONTAINERS_PODMAN_CMD=sudo -n podman --runtime=crun` and the [CLAUDE.md "Containers" prereq](CLAUDE.md) requires `sudo pacman -S crun`.
- ~~**Discovery → scraper auto-start wiring.**~~ Done — `cmd/server/main.go` wires `discovery.NewWatcher` `onAdd`/`onRemove` directly to `scrMgr.Start`/`Stop`, swallowing already-running errors so manual + watcher paths coexist.

## Milestone 4 — SvelteKit overlay + container management UI

**Status:** Ported. Containers admin UI at [sveltekit/src/routes/containers/](sveltekit/src/routes/containers/) — list/create/start/stop/delete table backed by the M3 admin endpoints, 3s status polling (paused when the tab is hidden), modal create form gated by a name regex, delete confirmation modal, external links to the per-instance xemu HTTPS port and browser kiosk port. Players overlay at [sveltekit/src/routes/overlays/players/](sveltekit/src/routes/overlays/players/) subscribes to the M2 scraper WebSocket via [sveltekit/src/lib/stores/scraper-ws.svelte.ts](sveltekit/src/lib/stores/scraper-ws.svelte.ts) and renders up to 4 local players with team-color stripes, K/D/A, health/shield bars, weapon + ammo (or energy charge), camo/overshield toggles, on a transparent background sized for OBS Browser Source. Layout config in [sveltekit/src/lib/config/layout.ts](sveltekit/src/lib/config/layout.ts) suppresses header/nav/toaster on `/overlays/*` so the overlay composites cleanly.

Admin-gating gotcha worth recording: the `isAdmin` field on `users` is declared `Hidden:true` in [internal/pocketbase/schema/users.go:53-58](internal/pocketbase/schema/users.go#L53-L58), so PocketBase strips it from the auth record returned to the client — meaning the SvelteKit guards saw `record.isAdmin === undefined` and admins were silently treated as non-admins (no Admin nav group, direct nav to `/containers/` bounced to home). Fix: extended [internal/pocketbase/routes/me.go](internal/pocketbase/routes/me.go) to expose `{isAdmin, isSuperuser}` for the authenticated caller, and [sveltekit/src/lib/stores/auth.svelte.ts](sveltekit/src/lib/stores/auth.svelte.ts) hydrates an `isAdmin` boolean (plus a `ready` promise) from `/api/me` on every token change. [sveltekit/src/lib/utils/guards.ts](sveltekit/src/lib/utils/guards.ts) now reads from the store. Field stays hidden — clients never see other users' admin status, and PB still blocks self-promotion via the standard collection PATCH path.

### M4 follow-ups (deferred)

- **Validate overlay in OBS Browser Source.** Smoke-tested in a normal browser tab; not yet pointed at OBS specifically. Should be a sanity check (transparent background, no scrollbars, font rendering at 1080p) once a Halo: CE match is set up.

## Milestone 5 — Scraper & WebSocket phase model + cache refactor

Restructure the scraper pipeline and the WS emission layer around a clear three-phase model (Idle / Ready / Live) with an authoritative per-instance cache as the source of truth. The current implementation works but conflates lifecycle, scrape cadence, broadcast wrapping, and _caching of pre-marshaled wire bytes_ in the runner; clients reconstruct state from a stream of envelopes rather than reading a coherent cached object on connect. This milestone introduces explicit phases, a structured cache (`instanceCache`), a per-instance room (`host:<name>`) + aggregate room (`host:all`), and a cleaner emission protocol that _builds_ envelopes from the cache on demand instead of replaying stale broadcast bytes.

Driving brief: [scraper-ws-refactor-brief.md](scraper-ws-refactor-brief.md).

**Honors existing conventions:** one-per-file registration, `internal/scraper/manager` and `internal/websocket/handlers` package layouts, `guards.Services` for cross-system access, and the M2c wrap-not-extend envelope policy (`Message{Type:"scraper", Room, Payload:<envelope>}`).

### Terminology

The word "snapshot" is **deliberately retired** by this refactor. Today it appears in three different roles in the codebase, and conflating them is part of why the wire model is hard to reason about:

- **Legacy: `snapshot` as an envelope type.** Today the runner broadcasts envelopes of type `"snapshot"`, `"tick"`, and `"event"`. After this milestone, those types are replaced by `current_state` (full cache contents) + `state_update` (per-scrape cadence cache update) + `event` (unchanged in spirit). The new wire protocol contains no envelope named "snapshot".
- **Legacy: `ReadSnapshot()` as a `GameReader` method name.** Today this method reads a mix of static (map, gametype, scenario data) and volatile (roster scores, team scores) fields. The method may keep the name for diff-readability or be renamed (see open question 3) — but the name is _internal_ and not meaningful to the wire protocol.
- **Legacy: `runner.latestSnapshotMsg` and `Manager.LatestSnapshotMessages()`.** These cache the marshaled `Message` bytes for the most recent `snapshot` envelope and replay them on `join_room`. Both are removed in 5a (cache becomes structured) and 5c (replay becomes a `current_state` build from the cache).
- **Generic English usage.** When a brief or doc says "atomic snapshot of state", the intended meaning is "an atomically-read consistent view of the cache". This milestone uses **"atomic cache read"** instead.

After this milestone, references to "snapshot" in code, comments, log lines, and docs should either disappear or be qualified by which legacy role they refer to. New names:

| Concept (new model)                                    | Term used here                            |
| ------------------------------------------------------ | ----------------------------------------- |
| Full cache contents emitted on join + phase transition | **`current_state` envelope**              |
| Per-scrape cadence update of the tick-fields portion   | **`state_update` envelope**               |
| Discrete happenings during Live (kills, pickups, etc.) | **`event` envelope**                      |
| The structured per-instance cache held by the runner   | **`instanceCache`**                       |
| The aggregated cross-instance summary cache            | **`hostsCache`** (drives `host:all` room) |
| An atomically-read consistent view of `instanceCache`  | **"atomic cache read"**                   |
| Live-match fields fixed for the duration of the match  | **"match-static fields"**                 |
| Live fields that change during play                    | **"tick fields"**                         |

### Out of scope

- **PocketBase persistence** of events / final match state — leaves the in-memory `previous_game` shape such that a future flush is straightforward (M13, formerly M5).
- **Halo 2 and other titles** — honor the registry extension point, don't build implementations.
- **Phase-transition debouncing** — emit transitions as observed; if title-ID reads turn out flappy during boot, debounce later.
- **Backpressure on slow clients** — current Hub behavior is fine for now.
- **Wire-format backwards compatibility** — no external consumers; SvelteKit updates alongside.

### 5a. Phase model + structured cache (scraper-internal, no wire change)

**Status:** Implemented. New [phase.go](internal/scraper/manager/phase.go) carries the `PhaseIdle / PhaseReady / PhaseLive` enum. The runner's old field cluster (one pre-marshaled-bytes cache for the legacy "snapshot" envelope plus several per-field caches) is replaced by a structured `instanceCache` in [runner.go](internal/scraper/manager/runner.go) holding phase, identity (`TitleID / Title / XboxName`), freshness (`LastReadAt / EngineTick / Iterations`), `GameData`, event log, and `PreviousGame` slot. [loop.go](internal/scraper/manager/loop.go) is now phase-driven (Idle ~3s → Ready ~500ms → Live ~30Hz tick-paced), with a `defer` that captures the just-ended match into `cache.PreviousGame` even on panic / ctx-cancel / heartbeat fallout. [manager.go](internal/scraper/manager/manager.go) `JoinReplayMessages()` builds the join-replay bytes on demand from the cache rather than caching pre-marshaled bytes. The `GameReader` interface renames `ReadSnapshot → ReadGameData`, `ReadLobby → ReadReadyState` (Halo: CE plugin updated). The "snapshot" term is fully retired internally — the only remaining references are the three `envelopeType*` constants in [loop.go](internal/scraper/manager/loop.go) holding the legacy wire-type strings until M5 stage 5c.

Also implemented in this stage:

- **OQ4 (single runner per instance lifetime, hot-swappable reader).** `Manager.Start` no longer calls `scraper.Detect` upfront — runners are created in Idle and self-detect via `scraper.ReadTitleID(r.inst)` + `scraper.Lookup(titleID)` on their own poll iterations. On unrecognised titles or detection drops the runner releases its reader and stays in Idle. `Manager.Start` now only fails on QMP init errors; the discovery watcher and `/api/admin/scraper/start` route comments are updated to match.
- **Phase + `LastReadAt` + `PreviousGame` exposed via Inspect.** `InspectState` gains `Phase` (string), `LastReadAt` (time), `PreviousGame` (game data + events + ended-at). The admin debug page's Overview tab status row renders all three so phase transitions are visible without inspecting the cached game data.
- **OQ6 heartbeat fallback (Live → Idle).** During Live, `liveReadFailureLimit` consecutive `ReadGameState` errors transition the runner back to Idle. Calibrated at ~300ms of failure (30 polls × 10ms), enough to ride out single-tick reads the engine missed but quick to react to a clean xemu exit.
- **OQ6 continuous-probe endpoint.** New `GET /api/admin/xemu/probe-title?sock=<path>&samples=<n>&interval_ms=<ms>` ([probe_title.go](internal/pocketbase/routes/xemu/probe_title.go)) samples the title-ID + XBE magic at GVA 0x00010000 over time. Investigation tool — run while transitioning Halo CE → quit-to-dashboard to determine whether the title-ID address flips reliably or stays stale. The heartbeat fallback above is the production behaviour while that investigation is pending; once the probe data is in, a more direct title-ID-based Live → Idle exit may replace or complement the heartbeat.
- **XBE-swap correctness fix.** `xemu.Instance.LowHVA` returns a cached HVA from the one-shot `Init`-time GVA→GPA→HVA translation. Across XBE swaps (dashboard → game, game → dashboard, game → game) the kernel keeps the guest VA but moves the underlying physical page, so the cached HVA reads stale bytes from the previous mapping — which would have made every Idle poll return the _first_ XBE's title ID forever. New `xemu.Instance.RefreshLowHVA(gva)` re-runs the QMP translation in place, and `scraper.ReadTitleID` now always re-translates so the Idle / Ready title-ID polls observe XBE swaps correctly. Same fix means the OQ6 probe captures fresh data on every sample rather than re-reading the start-time HVA.
- **Ready stuck-on-errors fix.** The Ready loop now runs the title-ID re-check _before_ `ReadGameState`, so when an XBE swap leaves Halo's reader pointing at stale / unmapped addresses (and `ReadGameState` returns errors every iteration), the runner still escapes to Idle within ~5s rather than looping forever on the failed read path.

**Smoke test (2026-05-05, validating 5a + 5b together):**

End-to-end exercise on three concurrent containerised instances (debug-host, debug-alpha, debug-bravo) all bound to a Halo: CE system-link lobby. Played a 2-kill Slayer match with a hard-quit ending. Phase + score timeline captured via 500ms inspect polling, kill chain captured via host:debug-host WebSocket subscription:

- **Idle → Ready** (UnleashX `0x9E115330` → Halo: CE `0x4D530004`): ≤3.04s (one Idle poll cycle).
- **Ready → Live** (lobby menu → in_game): fired exactly at first in_game tick.
- **Live phase**: ~90s of 30Hz tick broadcasts (536 snapshot envelopes), full kill chain captured (32 events: spawn / melee / damage / death / kill / score / kill_streak / team_score / game_start / player_joined). Damage-table → kill attribution working, kill_streak counters match per-player kill counts.
- **Live → Ready (postgame)**: cleanly captured; `cache.PreviousGame` populated with full roster (Whisp/Mopey on team 0, Sleepy on team 1 with 2 kills 0 deaths, matching the actual play), 32 events, ended_at timestamp.
- **Postgame → menu**: `prev_game=Y` correctly preserved across the cs change inside Ready.
- **Ready → Idle** (Halo: CE → UnleashX dashboard via xemu reset): captured in ≤504ms (lucky timing on the title-ID re-check; worst case ~5s gated by `readyTitleCheckInterval=10`). For non-host clients (debug-bravo) the engine first transitions in_game → menu (network drop), then Ready → Idle ~5s later when the title-ID re-check sees the XBE swap.
- **OQ6 answered without needing the probe**: the runner could only have escaped Ready by `scraper.ReadTitleID(r.inst)` returning the new value — there is no other Ready→Idle path. The clean transition validates that the M5 5a `RefreshLowHVA` fix re-translates the GVA correctly across XBE swaps. The `/api/admin/xemu/probe-title` endpoint stays in the tree as a future diagnostic.

**Lessons / findings worth keeping:**

- Hard-resetting xemu (vs in-game "Quit to Main Menu") bypasses the engine's `QuitFlag` and player-roster updates, so no `player_quit` event fires on hard quit. Pre-existing 5a behavior, captured below as a follow-up.
- The probe-title endpoint's response is built and returned all-at-once via `e.JSON`, so `curl` cannot be killed mid-window — kill it and you discard everything. For tight-window probes use a short bounded sample count (e.g. 60 × 500ms = 30s) so the response materialises in time.

- Introduce `Phase` enum (`PhaseIdle`, `PhaseReady`, `PhaseLive`) on the runner.
- Replace `runner.latestSnapshotMsg []byte` (legacy marshaled-bytes cache) with a structured `instanceCache` holding: phase, always-on values (title, Xbox machine name, freshness indicator, `last_successful_read_at`), match data (same field set across Ready and Live), event log (only meaningful in Live), `previous_game` slot (Ready-only, populated by Live → Ready transitions).
- Reshape the loop into phase-driven branches:
  - **Idle (~3s):** poll title ID + non-game-specific values via a new `ReadIdleData()` method on `GameReader` (or a non-plugin code path — see open question 3). Watch for title-ID becoming recognized.
  - **Ready (~500ms):** read the full match-data field set every poll (no static/tick split yet). Reuses today's `ReadLobby()` (cheap variant) — see open question 3 for whether to rename.
  - **Live (~30Hz tick-paced):** read match-static fields once on Ready → Live (cached for the match), tick fields every tick. Reuses today's `ReadSnapshot()` for the static-fields read and `ReadTick()` for tick fields — see open question 3.
- Implement phase transitions including the Live → Ready cleanup that moves the just-ended match into `previous_game`. Use `defer` so a panic / ctx cancel / xemu crash mid-match still moves data rather than dropping it.
- Atomic cache reads: mutex-protected pointer swap so a reader (later, the WS layer responding to `request_state`) sees a complete `instanceCache`, not a half-built one.
- `last_successful_read_at` advances on every successful read; failed reads logged but do not advance the timestamp.
- Keep broadcasting today's `Message{Type:"scraper", Room:"overlay", Payload:<envelope>}` shape (and today's legacy `snapshot`/`tick`/`event` envelope types) so the SvelteKit overlay continues to work — no wire change yet.

**Why first:** every other stage depends on a structured cache existing. Locking down the phase machine and the cache shape _before_ changing the wire keeps the diff readable and lets us verify the in-memory model in isolation against a live xemu.

**Defers:** wire-format envelope shape changes, room-name changes, addressed-reply handlers, frontend updates.

**Investigation work folded in:** answer open question 6 (Live → Idle detection gap) during this stage by adding a `/api/admin/scraper/{name}/probe-title` endpoint or extending the existing probe to dump the XBE title ID continuously, compare across the Halo CE → dashboard transition, and either fix the read or document why title-ID-based load-out detection is unreliable and propose an alternative (state-poll heuristic, QMP signal, or process-restart watch).

### 5b. Multi-room model + reserved-name chokepoint

**Status:** Implemented and live-validated (see 5a smoke-test block above — 5a + 5b were tested together against the same Halo: CE session). The single shared `overlay` room is gone; per-instance broadcasts target `host:<name>` and a new aggregator goroutine drives a cross-instance `host:all` summary feed.

What landed:

- **Chokepoint + RoomType** in [internal/websocket/rooms/host.go](internal/websocket/rooms/host.go): single `host` RoomType registered (RequireAuth) — `host:smoke1` and `host:all` both resolve to it via the registry's existing prefix-strip logic (separate `host:all` registration would be unreachable, documented in the file). The reserved suffix `"all"` and any name containing `:` or whitespace are rejected by the exported `RoomForInstance(name) (string, error)` chokepoint, which is the single trust boundary for instance-name → room-name derivation.
- **Aggregator** in [internal/scraper/manager/aggregator.go](internal/scraper/manager/aggregator.go): one goroutine per Manager owns the `host:all` writes. `mutex+map[string]hostSummary` storage holding `{instance, phase, title, map, gametype, score_summary, last_successful_read_at}` per instance; non-blocking buffered-channel `post()` from runners; 250ms coalesce ticker bounds the broadcast cadence. Full re-broadcast on every dirty tick (OQ2 — no diffs); `host:all` envelope's `instance:"all"` is the client-side disambiguator from per-instance feeds.
- **Runner wiring**: each runner caches its `hostRoom` string at construction (validated by the chokepoint at `Manager.Start`) and posts hostSummary updates via `publishSummary()` on phase changes + game-data changes + a 1s heartbeat (`maybeHeartbeatSummary` wired into `recordIteration`).
- **JoinReplay extensions** in [internal/guards/interfaces/scraper/joinreplay.go](internal/guards/interfaces/scraper/joinreplay.go): `JoinReplayForInstance(name)` and `JoinReplayForHostAll()` added; the legacy `JoinReplayMessages()` survives until 5d narrows it. The [join_room handler](internal/websocket/handlers/join_room.go) dispatches replay per room.
- **Defense in depth** in [internal/discovery/watcher.go](internal/discovery/watcher.go): the watcher logs and skips any `all.sock` to avoid spamming a doomed start every poll, even though `Manager.Start` is the actual trust boundary.
- **Frontend minimal update** in [sveltekit/src/lib/stores/scraper-ws.svelte.ts](sveltekit/src/lib/stores/scraper-ws.svelte.ts): client now joins `host:all` first, then auto-subscribes to each `host:<name>` returned in the summary payload (via a `SvelteSet` to satisfy `svelte/prefer-svelte-reactivity`). The legacy `firstGameData`/`firstTick` accessors keep the overlay rendering unchanged.
- **HTTP status code fix** in [internal/pocketbase/routes/scraper/handlers.go](internal/pocketbase/routes/scraper/handlers.go): chokepoint rejections (`name="all"` etc.) now return `400 Bad Request` instead of `502 Bad Gateway` via a new `manager.ErrInvalidName` sentinel. True upstream failures (QMP init) still return 502; name collision still 409.
- **Tests**: `host_test.go` covers `RoomForInstance` accept/reject + verifies `host:smoke1` and `host:all` both resolve to the registered host RoomType. `aggregator_test.go` covers coalesce-on-dirty-tick, idle-no-broadcast, Removed eviction, full-snapshot rebroadcast, joinReplay envelope shape, and the team-score formatting helper. `manager_test.go` extended for the chokepoint enforcement at `Start`.

**Defers (unchanged):** envelope shape changes are 5c; addressed-reply narrowing of `request_state` is 5d; per-instance subscription UI flow + per-overlay route param wiring is 5e.

### 5c. Emission protocol (envelope shapes + ordering)

**Status:** Implemented. The legacy `snapshot` / `tick` / `event` wire-type set is replaced by `current_state` / `state_update` / `event` across all per-instance and host:all paths. New `CurrentStatePayload` and `StateUpdatePayload` types in [runner.go](internal/scraper/manager/runner.go) define the wire shape; `runner.buildCurrentStateEnvelope()` and `runner.buildStateUpdateEnvelope(phase)` are the single source for marshaled bytes (used by both the loop broadcast paths and the join-replay paths). `loop.go`'s dispatcher emits `current_state` on every phase transition before the new phase function starts emitting `state_update`s — the runner being the single goroutine writer for its `host:<name>` room satisfies the brief's ordering invariant for free. The aggregator's `host:all` envelope is now `current_state` carrying the full `[]hostSummary` (per OQ2's "full re-broadcast, no diffs"). `Manager.JoinReplayMessages()` and `JoinReplayForInstance()` now build `current_state` envelopes from the full `instanceCache`, so a late-joining client gets phase + identity + freshness + game data + recent events + `previous_game` in one message rather than just GameData.

Frontend (SvelteKit) consumes `'snapshot'` / `'tick'` and is broken on this branch by design — 5e ships the matching client update; 5d narrows `request_state` to a single-room reply and adds `request_events`.

Unit tests in [wire_test.go](internal/scraper/manager/wire_test.go) cover the builders across all three phases (Idle has no per-phase payload + envelope tick=0; Ready carries `Ready` game data + tick=0; Live carries `Tick` payload + envelope tick=engine tick) and the postgame Ready-with-PreviousGame case.

- Replace today's legacy `snapshot` / `tick` / `event` envelope set with the new protocol:
  - **`current_state`** (per-instance room): full `instanceCache` contents (phase, always-on values, match data, event log, `previous_game` if present). Sent on join (replacing today's `LatestSnapshotMessages` replay in `join_room`) and on every phase transition.
  - **`state_update`** (per-instance room): the tick-fields portion of the cache, sent every scrape during all three phases at phase-appropriate cadence. Carries phase, instance, and tick (where meaningful — see "decisions made").
  - **`event`** (per-instance room): instance + tick + event type + metadata. Streams independently of `state_update` — a scoreboard client reads `state_update`, a kill-feed client reads `event`, both are valid.
  - **`current_state`** (default room `host:all`): full `hostsCache` on join (list of per-instance summaries).
  - **Default-room update** (`host:all`): full updated `hostsCache` re-broadcast on any instance summary change or instance add/remove.
- Enforce the ordering rule: the new `current_state` for a phase transition reaches clients before any `state_update` envelope tagged with the new phase. Single goroutine per room (already a runner invariant) makes this trivial to guarantee.
- Update the [`join_room` handler](internal/websocket/handlers/join_room.go) to emit `current_state` for whichever `host:*` room the client just joined, built fresh from the `instanceCache` (or `hostsCache` for `host:all`) from 5a — no pre-marshaled-bytes cache anymore.

**Why third:** depends on (5a) the cache existing and (5b) rooms being registered. With both in place, this stage is a focused "redefine the wire" change rather than a multi-domain refactor.

**Defers:** client-side handling of new envelope shapes (frontend updates land in 5e), addressed-reply handlers (5d).

### 5d. Addressed-reply handlers (`request_state`, `request_events`)

**Status:** Implemented. [request_state.go](internal/websocket/handlers/request_state.go) now narrows replies to the requester's own `host:*` memberships (looked up via `e.Services.WS.UserRooms(e.UserID)`); `host:all` membership replays the host:all `current_state`, and each `host:<name>` membership replays the per-instance `current_state` from that runner's cache. New [request_events.go](internal/websocket/handlers/request_events.go) handler accepts an optional `{since_tick, types}` filter payload, iterates the requester's `host:<name>` rooms (skipping `host:all`), and sends one envelope per instance via `e.SendRaw`. The reply uses a new inner envelope type **`events`** (plural — distinct from per-event live `event` envelopes so clients can pattern-match request replies) carrying `{phase, since_tick, events}`. The cache-side filter + marshaling lives on the manager as `Manager.EventsReply` (added to the `scraperiface.Service` aggregate interface via a new `EventsReply` sub-interface) so the WS handler — which can't import `internal/websocket` without a cycle — gets pre-marshaled wire bytes back.

OQ1 resolution: in Idle and Ready phases the reply is always an empty events list, even when `previous_game` exists; the phase field tells the client why. OQ7 resolution: events are returned in oldest-first stream order — the cache stores newest-first, the filter reverses on the way out.

Unit tests in [events_test.go](internal/scraper/manager/events_test.go) cover `filterEvents` (empty input, oldest-first reversal, `since_tick` filter, type-set filter) and `Manager.EventsReply` (unknown instance, Idle returns empty even with cached events, Live filters by tick + type and emits the `events` envelope shape).

Frontend wiring (resync calls + reply handling) lands in 5e.

**Bugfix surfaced during 5c+5d+5e smoke test (2026-05-06):** the original `filterEvents` matched the user's `types` parameter against `ev.Type`, but every detector in [internal/scraper/haloce/events/events.go](internal/scraper/haloce/events/events.go) emits envelopes with `Type` set to the literal `"event"` wire-type — the semantic event type lives in `payload.event_type`. The unit test built synthetic envelopes with `MakeEnvelope(typ, ...)` so it passed without exercising the runtime shape. End result: `request_events` with any non-empty `types` list returned zero events. Fixed by extracting `event_type` from the payload JSON in `eventInnerType` and matching against that, plus updating the test fixture to use the runtime envelope shape. The frontend's `eventBucket` had a parallel bug — it read `ev.payload.type` (the inner field is `event_type`) and fell through to `ev.type` which is always `"event"`, so every event ended up in the `"other"` bucket. Fixed in [sveltekit/src/routes/admin/debug/[name]/+page.svelte](sveltekit/src/routes/admin/debug/[name]/+page.svelte).

- Update [internal/websocket/handlers/request_state.go](internal/websocket/handlers/request_state.go) to look up the `host:*` room the requester is in and reply with the new `current_state` envelope for that room (via `e.SendRaw` so it's addressed only to the sender). Today's handler returns legacy snapshot bytes for all instances — narrow it to a single `current_state` build for the requester's room.
- Add `request_events` handler. Optional filters: `since_tick` (events with `tick > N`, for resync after a connection gap) and `types` (string-array filter). With no filters, return the full Live-phase event log. **In Idle and Ready, return an empty list even when `previous_game` exists** (resolves open question 1). Reply via `e.SendRaw`.
- Document the ordering convention for `request_events`: events returned in the same order they were appended to the live event stream (open question 7).

**Why fourth:** depends on (5c) for envelope shapes; the new `request_state` reply has to use the new `current_state` shape.

**Defers:** any persistence-backed event lookup (out of scope; in-memory only).

### 5e. SvelteKit client update

**Status:** Implemented. The `scraperWS` store in [sveltekit/src/lib/stores/scraper-ws.svelte.ts](sveltekit/src/lib/stores/scraper-ws.svelte.ts) is rebuilt around per-instance state records keyed by host name (`gameData[name]`, `ticks[name]`, `events[name]`, `phases[name]`, `hostSummaries[name]`, `previousGames[name]`, etc.). Type guards `isCurrentState` / `isStateUpdate` / `isEvent` / `isEventsReply` discriminate the new envelope set in [sveltekit/src/lib/types/scraper.ts](sveltekit/src/lib/types/scraper.ts). The store exports `requestState()` and `requestEvents({sinceTick, types})` which the per-instance debug page calls on connect + reconnect for gap-fill resync. The instance-picker on [admin/debug/+page.svelte](sveltekit/src/routes/admin/debug/+page.svelte) consumes `host:all` summaries to render phase + score live across all running instances; the per-instance page on [admin/debug/[name]/+page.svelte](sveltekit/src/routes/admin/debug/[name]/+page.svelte) subscribes only to its own `host:<name>` room and merges the WS feed against the 3s HTTP inspect poll as a fallback. Players overlay at [sveltekit/src/routes/overlays/players/+page.svelte](sveltekit/src/routes/overlays/players/+page.svelte) uses the `firstGameData` / `firstTick` convenience accessors so a single overlay surface still works without per-instance routing.

`pnpm check` clean (0 errors, 0 warnings). `pnpm lint` clean. `pnpm test` 11/11 pass.

**5c+5d+5e smoke test (2026-05-06):**

End-to-end exercise across four containerised instances (debug-host as neutral host, debug-alpha/bravo/charlie as players) all in a Halo: CE system-link match. Captured wire shapes for each phase via a Python websocket-client harness against `/api/ws`:

- **Idle** (debug-host on UnleashX dashboard before disc insert): `current_state` envelope on join carried full identity block (xbox_name, serial_number, mac_address, video_standard, kernel_system_time, kernel_boot_time, kernel_uptime_ns, time_zone_std/dlt, xbe_title_name, xbe_version, xbe_game_region, xbe_allowed_media), no game_data, envelope.tick=0. `state_update` cadence: 1/5s ≈ ~3s as designed.
- **Ready** (debug-bravo at lobby/menu): `current_state` carried identity + game_data (machines / players / map / gametype / power_item_spawns), envelope.tick=engine_tick=296819 at build time. `state_update` cadence: 10/5s ≈ ~500ms; payload had `phase + freshness + ready` (the GameData under the `ready` field), envelope.tick=0 (correct — non-Live).
- **Live** (all 4 in match): `state_update` cadence: 300/10s = exactly 30Hz across each instance ✓; payload had `phase + freshness + engine_tick + tick` with full TickPayload (game_globals, locals, network, objects, players, power_items). `current_state` carried identity + game_data + latest_tick + accumulated events. envelope.tick on Live `state_update` = engine_tick (1961, 1962, …, advancing). Aggregator broadcast `host:all` `current_state` 21/10s ≈ 2.1/s (250ms coalesce ticker firing on dirty Live ticks across 4 instances) carrying the full `[]hostSummary`.
- **Phase-transition ordering:** for every room, exactly one `current_state` arrived on join before any `state_update` tagged with the new phase ✓.
- **`request_state` addressed reply:** one `current_state` per host:\* room the requester is in (host:all + each host:<name>) — replies carried fresh atomic-cache reads, not stale broadcast bytes.
- **`request_events` plural envelope:** type=`"events"` (distinct from streaming `"event"`), payload `{phase, since_tick, events}` ✓. Idle and Ready returned empty even when `since_tick` would otherwise match cached events (OQ1 ✓). `since_tick` filter respected — at since_tick=2529 all replies returned only events with tick > 2529 (min_event_tick=4081). Type filter required the bugfix above before it returned events; post-fix `types=['spawn']` returned 4 spawn events at the most recent respawn tick, `types=['kill']` returned 0 (no kills yet in match), `types=['event']` (the literal wire type) correctly returned 0 since no event has `event_type:"event"`.
- **Chokepoint:** `POST /api/admin/scraper/start name=all` returned HTTP 400 with `ErrInvalidName` sentinel; `name="foo:bar"` likewise rejected with the colon-rejection error. ✓

Test artifacts (Python harness scripts) live at `/tmp/m5_*.py` for re-running.

**Not validated end-to-end this run** (covered by unit tests in [wire_test.go](internal/scraper/manager/wire_test.go) + the prior 5a/5b 2026-05-05 smoke test): Live → Ready transition populating `previous_game`, hard-quit Live → Idle via heartbeat fallback, streaming `event` envelopes (cache events were verified via `request_events`; the streaming path uses the same `r.broadcast(svc, ev)` builder with the same envelope shape).

- Replace single hardcoded `join_room` to `"overlay"` with per-instance subscription: pages join `host:<instance>` based on the route param, the admin debug page can subscribe to multiple, the instance-picker UI subscribes to `host:all`.
- Replace today's legacy `Envelope = {snapshot|tick|event}` consumer with the new envelope set: `current_state`, `state_update`, `event`.
- Wire `request_state` and `request_events` into the store for resync after a connection gap.
- Update [scraper-ws.svelte.ts](sveltekit/src/lib/stores/scraper-ws.svelte.ts), the type definitions in [scraper.ts](sveltekit/src/lib/types/scraper.ts), the players overlay, and the admin debug page tabs.

**Why last:** all backend wire-format work must land first; the frontend churn is contained here and ships in lockstep with the new protocol.

**Defers:** persistence-backed history views (M13, formerly M5).

### Open questions (with proposed resolutions)

1. **`request_events` outside Live** — _Resolved by brief._ Returns empty in Idle and Ready, even when `previous_game` exists. Rationale: a client asking for events shouldn't have to also check phase to know whether the response is "live" or "from a finished match". If post-game replay becomes useful later, expose via a separate `request_previous_game` message.

2. **Default-room update granularity** — _Proposed: full list re-broadcast._ Aligns with the brief's recommendation. Payload is small (a handful of summary records); diff logic is more complex and won't pay off until we have many more instances. Revisit if the per-instance summary grows or the host count grows large.

3. **`GameReader` interface evaluation** — _Proposed: minimal extension + rename._ Reading [internal/scraper/haloce/reader.go](internal/scraper/haloce/reader.go) shows the existing methods map cleanly: `ReadSnapshot` already caches scenario-static data and re-reads volatile fields on each call (matches Live's static-fields-cached + Ready's full reread); `ReadLobby` is the explicit cheap variant for non-in_game (matches Ready's cadence); `ReadTick` matches Live's tick reads. The only misfit is **Idle**, which today reads the ambient game state via `ReadGameState()` but has no notion of "Xbox machine name + freshness indicator". Proposed interface changes:
   - **Add `ReadIdleData() (IdlePayload, error)`** returning `{title?, machine_name, clock_or_freshness_value, last_read_at}`.
   - **Rename `ReadSnapshot` → `ReadMatchState`** (or `ReadFullState`) to retire the overloaded "snapshot" term. The method name is internal — no wire impact — and the rename clarifies that it reads the match-data field set (static + volatile), distinct from the wire-protocol `current_state` envelope. Consistent with the brief's explicit retirement of the "snapshot" term.
   - **Rename `ReadLobby` → `ReadReadyState`** (or `ReadActiveState`) for the same reason — "lobby" is one of several Ready-phase contexts (lobby, post-match stat screen, between-match menu).
   - Keep `ReadTick`, `ReadGameState`, `OnStateChange`, `BuildScoreProbe`, `LastStateInputs`, `NewTickState`, `XboxName`, `Title`, `LowGVAs` unchanged.
   - Do **not** predeclare a separate static/tick method split on the interface — the existing `ReadSnapshot`/`ReadTick` pair plus internal scenario caching in the plugin (today's pattern) already deliver the right behavior.

   Renames are mechanical and can land in 5a alongside the cache work, or as a prep-stage 5a-prelude commit, depending on how clean the diff needs to be.

4. **Idle-phase scraping mechanics** — _Proposed: single runner per instance lifetime with a hot-swappable reader._ Today `Manager.Start` fails when `scraper.Detect` doesn't recognize the title — no runner is created and the discovery watcher logs the failure. New model: `Manager.Start` always creates a runner; the runner owns the `*xemu.Instance` for the whole socket lifetime. The runner enters Idle with no `GameReader`. On title-ID becoming recognized, the runner loads the matching reader (registry lookup) and transitions to Ready. On title-ID becoming unrecognized (Live → Idle or Ready → Idle), the runner drops the reader and returns to Idle. Justification: keeps lifecycle tied to socket presence (matches discovery's mental model), avoids the complexity of two runner classes and handoff between them, and leaves a clean place for the Idle-phase poll loop.

5. **Reserved-name enforcement for `all`** — _Proposed: confirm chokepoint approach._ Single function `roomForInstance(name) (string, error)` in [internal/websocket/rooms/](internal/websocket/rooms/) is the only sanctioned way to derive a room name from an instance name. Returns error on `name == "all"` (or anything that contains `:` or other reserved characters). Every code path that needs a room name goes through it. PocketBase API rules and podman create-validation can layer on top for user-facing rejection at create time, but the chokepoint is the trust boundary. The discovery watcher in [internal/discovery/](internal/discovery/) needs a small change to filter out `.sock` files whose stem is `all` so the chokepoint never sees that name from disk.

6. **Load-out detection gap** — _Proposed: investigate during 5a, two candidate causes._ The current [loop.go:168-181](internal/scraper/manager/loop.go#L168-L181) periodic check (every ~5s during idle states only) compares `scraper.ReadTitleID(r.inst)` against the start-time `r.titleID`. Two candidate causes for the Halo CE → dashboard miss:
   - (a) The XBE header at GVA `0x00010000` retains the old title ID after game exit because xemu doesn't re-load that page when the dashboard takes over.
   - (b) The check only runs in idle states; if the runner is in-game when the user quits, no Live → Ready transition fires (the game can't emit it if it's gone).

   Investigation plan: add a debug probe that continuously reads the title ID + a dashboard-detection heuristic (e.g., presence of an XBE-magic check, a known dashboard title ID, or a memory-region nullity test) across a real Halo CE → quit-to-dashboard transition; pick the most reliable signal. If the title-ID address is the right place but the read is stale, propose re-translating the GVA on each check (xemu may have remapped the page). If the title-ID address is unreliable, fall back to a state-machine signal (e.g., heartbeat: if `ReadGameState` errors for N consecutive polls, assume Live → Idle). Document the finding either way.

7. **Event ordering for `request_events`** — _New question, not in brief._ Proposal: events returned in the same order they were appended to the live event log (registration / detection order, which today is per-detector iteration order in [internal/scraper/haloce/events](internal/scraper/haloce/events)). Document that ordering is "stream order, not necessarily strict tick order" so a client doing post-hoc analysis knows not to assume `event[i].tick <= event[i+1].tick` for events from the same tick.

8. **Multi-runner writes to `host:all`** — _New question._ Many runners may push summary updates to the aggregate room. Proposal: a single aggregator goroutine owns `host:all` writes; runners post `summaryUpdate` events to a buffered channel, the aggregator coalesces and broadcasts. Keeps the "single goroutine writes per room" invariant intact and avoids lock contention between runners. Aggregator lives in `internal/scraper/manager/` next to the per-instance runners.

### Decisions made (where the brief was internally inconsistent or open)

- **Idle scrape cadence**: brief says ~3s; existing loop uses 500ms even in non-game states. Decision: follow brief (3s in Idle, ~500ms in Ready). The 500ms current value was tuned for menus / lobbies (which are now Ready), and Idle is genuinely "we have nothing useful to read until the title changes" — 3s is sufficient.
- **Retire vs alias the `overlay` room name**: brief says "the current single shared `overlay` room goes away" but doesn't specify whether to alias it briefly during the rollout. Decision: hard switch in 5b — there are no external consumers and the SvelteKit client is updated in 5e of this same milestone, so an alias buys nothing.
- **`tick` field semantics on `state_update`**: brief says "Carries the phase, instance, and tick (where meaningful) on every envelope". Decision: `tick` is omitted (or `0`) outside Live; in Live it's the engine tick. Documented on the envelope type.
- **Default-room name**: brief uses both `host:all` ("Room model" section) and just describes "the default room". Decision: `host:all`, matching the per-instance prefix.

### Smoke test (post-implementation, runs after 5e)

1. `task dev` with `CONTAINERS_ENABLED=true`.
2. Create + start two containers (`smoke1`, `smoke2`) via the M3 admin endpoints; both appear in `host:all`'s `current_state` as Idle until xemu finishes booting.
3. With Halo CE not yet inserted, both should report Idle phase + Xbox machine name + advancing freshness indicator in their `host:<name>` room's `current_state`. Verify in the admin debug page (after 5e wires the per-instance tabs to per-instance rooms).
4. Insert Halo CE on `smoke1` → Ready transition. Confirm a fresh `current_state` reaches subscribers before any `state_update` tagged Ready arrives. Lobby fields populate within ~500ms.
5. Start a match on `smoke1` → Live at 30Hz; `state_update` envelopes carry tick fields at engine cadence; `event` envelopes stream independently as kills/etc happen.
6. Quit to Halo CE main menu mid-match → Live → Ready; `previous_game` populated in the `instanceCache`; `state_update` cadence drops to ~500ms; `request_events` returns empty (per open question 1).
7. Quit to xemu dashboard → Ready → Idle (this is the path open question 6 must fix); `previous_game` dropped; `state_update` cadence drops to ~3s.
8. Disconnect a WebSocket client mid-match, reconnect, send `request_state` → addressed-reply with one `current_state` for the requester's room; send `request_events` with `since_tick=<last_seen>` → addressed-reply with the gap.
9. `host:all` `current_state` re-broadcasts on every instance summary change; both instances stay represented throughout.

### M5 follow-ups (deferred)

- **`game_end` / `player_quit` synthesis on hard-quit paths.** Surfaced during the 5a + 5b 2026-05-05 smoke test. When xemu is hard-reset (vs an in-engine "Quit to Main Menu"), Halo CE has no opportunity to set the per-player `QuitFlag` byte the existing detector watches, so the cache never records why the match ended. Two approaches:
  - In `runLive`'s exit path (state `in_game` → anything else), synthesize one `game_end` event and append it to the cache before returning `PhaseReady`. Tiny code change in [internal/scraper/manager/loop.go](internal/scraper/manager/loop.go).
  - On Live → Idle via the heartbeat fallback (xemu vanished mid-match), synthesize a `player_quit` for every player still in the live roster. Slightly larger; touches the same exit paths.
    Both are M19-class robustness work — not blockers for 5c. File the deferred note here.

## Milestone 6 — Frontend polish (theme + auth-refresh fix + debug revamp)

> The site has accumulated style drift, the debug page needs both more data and better organization, and a long-standing auth-hydration race redirects admin pages to home on hard refresh. Cluster these three so the visual + dev-tooling foundation is solid before bigger feature work (M9 kiosk, M10 overlays).

### 6a. Auth-refresh redirect fix (frontend + backend audit)

Hard-refreshing `/containers/` or `/admin/debug/` bounces to home.

*Frontend root cause (primary).* In [sveltekit/src/lib/stores/auth.svelte.ts](sveltekit/src/lib/stores/auth.svelte.ts), `auth.ready` is captured at module init, but on a cold CSR hydration the `+page.ts` load function calls `requireAdmin()` ([sveltekit/src/lib/utils/guards.ts](sveltekit/src/lib/utils/guards.ts)) before the auth store has hydrated `isAdmin` from `/api/me`. Both [routes/admin/debug/+page.ts](sveltekit/src/routes/admin/debug/+page.ts) and [routes/admin/debug/[name]/+page.ts](sveltekit/src/routes/admin/debug/[name]/+page.ts) `await auth.ready` already, but the promise resolved instantly with stale state. Options: (a) move the `/api/me` fetch into a root `+layout.ts` load that all child routes inherit, (b) make `auth.ready` rebuild on every `pb.authStore` change rather than capture once. Decide during implementation.

*Backend gating audit.* Sweep [internal/pocketbase/routes/](internal/pocketbase/routes/) for parallel inconsistencies — guards that should fire but don't, or fire too eagerly. Focus on:
- [internal/pocketbase/routes/middleware/auth.go](internal/pocketbase/routes/middleware/auth.go) and [admin.go](internal/pocketbase/routes/middleware/admin.go) — confirm the `RequireAuth` + `RequireAdmin` chain matches what's documented in CLAUDE.md (PB superusers + `users.isAdmin=true`).
- [internal/pocketbase/routes/admin/](internal/pocketbase/routes/admin/) group registration — every admin route should inherit the chain, not re-add or skip.
- [internal/pocketbase/routes/me.go](internal/pocketbase/routes/me.go) — currently exposes `{isAdmin, isSuperuser}` to the caller; verify it doesn't leak other users' admin status.
- [internal/pocketbase/routes/containers/](internal/pocketbase/routes/containers/), [scraper/](internal/pocketbase/routes/scraper/), [xemu/](internal/pocketbase/routes/xemu/) — confirm each registered handler is gated.
- [internal/pocketbase/routes/allroutes.go](internal/pocketbase/routes/allroutes.go) and [allgroups.go](internal/pocketbase/routes/allgroups.go) — registration order.

If the frontend fix exposes a backend route that should have been guarded but wasn't, fix both at once.

Smoke test: hard-refresh `/containers/` and `/admin/debug/<name>/` while logged in as admin → page loads, no home bounce. Hit each `/api/admin/*` endpoint as anon, as a non-admin user, and as an admin → 401/403/200 respectively.

### 6b. Custom Skeleton theme + style consistency

Today the cerberus theme is loaded statically via [sveltekit/src/routes/layout.css](sveltekit/src/routes/layout.css) (`@import '@skeletonlabs/skeleton/themes/cerberus'`) and the root sets `data-theme="cerberus"` in [sveltekit/src/app.html](sveltekit/src/app.html). Define a project-branded theme — likely via Skeleton v4's theme generator or a hand-written `tailwind.config.ts` with custom design tokens. Audit pages for inconsistent spacing, button variants, and card patterns; centralize repeating chrome into reusable components. Pages to audit: `/`, `/login/`, `/containers/`, `/containers/[name]/`, `/admin/debug/`, `/admin/debug/[name]/`, `/overlays/players/`.

Smoke test: visual diff before/after; dark + light mode both render cleanly; OBS overlay backdrop stays transparent (overlays must remain unaffected by theme background).

### 6c. Debug page revamp + scraped-data validation

Existing tabs (Overview / Game / Tick / Events / Probe / Raw JSON) in [sveltekit/src/routes/admin/debug/[name]/+page.svelte](sveltekit/src/routes/admin/debug/[name]/+page.svelte) and components in [sveltekit/src/lib/components/debug/](sveltekit/src/lib/components/debug/). Scope:

- **Data coverage audit.** Walk every field surfaced by the Halo: CE reader ([internal/scraper/haloce/reader.go](internal/scraper/haloce/reader.go), `offsets.go`, `offsets_reference.go`); confirm each renders somewhere in the debug UI. Promote currently-buried fields where useful.
- **Restyle.** Apply M6b theme; replace `KvCard` / raw JSON dumps with structured tables for high-volume fields (objects, projectiles).
- **Verification harness.** For fields tagged `unverified` in the offset table, surface a per-field "looks plausible / clearly broken / unknown" annotation as a manual-validation pass, feeding M19's runtime offset validation.
- **Probe tab.** Audit the existing probe outputs (`BuildScoreProbe`, `LastStateInputs`) and add probes for any field cluster currently untrusted.

Smoke test: 4-instance system-link match (same harness as M5 5c+5d+5e smoke), walk every tab on every instance, log any field that displays empty/zero/garbage and create offset-investigation follow-ups for M19.

## Milestone 7 — Identity schemas: gamertags + teams

> Foundation for the match-aware kiosk (M9) and persistence stack (M13+). Many real users carry multiple gamertags ("Stewball32" / "Stewball" / "Stewie"); some rotate teams across events. Model this directly rather than forcing a 1:1 user↔gamertag↔team flattening.

### 7a. Schema design

Target shape:

- `users` (existing) — gains `default_gamertag` FK (nullable) so the system has a sensible "show me as" pick when one is needed and the user hasn't otherwise specified.
- `gamertags` (`user`, `tag`) — one row per (user, gamertag-string) combo. Simple two-column join. Optional fields to consider: `xbox_machine_name?` (if a tag is tied to a specific console), `notes?`. The `default_gamertag` FK on `users` lives there rather than as an `is_primary` flag on `gamertags` so there's a single canonical "default per user" with no risk of zero or multiple primaries; admins can change it via the user record.
- `teams` (`name`, `slug`, `created_by`).
- `trosters` — `team`, `gamertag`, `is_captain`, `is_manager`, `joined_at`, `is_active`. Both `is_captain` and `is_manager` are independent booleans (a player can be both, either, or neither). The roster row joins on `gamertag` (not `user`) so one user can rep different teams under different handles.

Decisions to lock during 7a: cascade rules on user deletion (soft-delete preferred so historical roster + game records survive), uniqueness constraint on `tag` (per-user unique; globally non-unique because two users can validly use the same handle on different consoles), whether `xbox_machine_name` belongs on `gamertags` or its own join table (recommend on `gamertags` until a real second-console-per-tag use case appears).

### 7b. PocketBase collections + admin UI

Add collections under [internal/pocketbase/schema/](internal/pocketbase/schema/), one file each (`gamertags.go`, `teams.go`, `trosters.go`). Update the existing users schema to add `default_gamertag`. Build a SvelteKit admin/self-service UI under [sveltekit/src/routes/admin/identity/](sveltekit/src/routes/admin/identity/) (or similar) for CRUD on gamertags + teams + trosters. Enforce row-level rules in PB API rules: a user can manage only their own gamertags + their own `default_gamertag` pick; team captains/managers can manage their team's trosters; everyone can read non-sensitive fields.

### 7c. Identity exposure to scraper + WS layers

Extend [routes/me.go](internal/pocketbase/routes/me.go) (per the M4 pattern) to include the caller's gamertag list + default. Surface a `gamertag` lookup helper through `guards.Services` (probably via a new `internal/guards/interfaces/identity/` aggregate) so handlers can answer "is this gamertag X owned by user Y?" without circular imports. No backend persistence of in-game events yet — that's M13.

Smoke test: create user → add 3 gamertags → set one as default → create 2 teams → attach different gamertags to each team with captain/manager flags set → confirm `/api/me` returns the membership graph; admin UI round-trips create/edit/delete; non-admin user can't touch other users' gamertags; a non-captain/manager can't edit their team's roster.

## Milestone 8 — Roles + permissions

> Today `users.isAdmin` is a single boolean ([internal/pocketbase/schema/users.go:53-58](internal/pocketbase/schema/users.go#L53-L58)) — fine for "is this person staff" but not enough as the surface area grows (tournament organizers, content moderators, stat reviewers, guild bot operators, etc.). Add a roles collection so permissions can be granted in named bundles, retire the bare `isAdmin` flag in favor of a role-membership check, and update the guard layer to consume roles instead of the boolean.

### 8a. Schema

New collections:

- `roles` (`slug`, `label`, `description`, `level: int`) — examples: `superuser`, `admin`, `tournament_organizer`, `team_manager`, `content_moderator`, `stat_reviewer`. `level` gives a coarse comparable rank for "is X at least an admin?" checks; finer-grained checks use slug membership.
- `user_roles` (`user`, `role`, `granted_by`, `granted_at`) — join table; a user can hold multiple roles.
- `permissions` (`slug`, `description`) and `role_permissions` (`role`, `permission`) — optional, for finer-grained "can_create_tournament", "can_post_to_discord_channel" style checks. Decide during 8a whether v1 ships with permissions tables or just role slugs (recommend role-slugs only for v1; defer permissions tables until a real use case demands them).

### 8b. Migrate `isAdmin`

Backfill a `roles` seed (`superuser` `level=100`, `admin` `level=50`, `member` `level=10`); migrate every existing `users.isAdmin=true` user into a `user_roles` row pointing at `admin`. Drop `users.isAdmin` from the schema and any code that reads it.

### 8c. Guard layer update

Replace [internal/pocketbase/routes/middleware/admin.go](internal/pocketbase/routes/middleware/admin.go) `RequireAdmin()` with `RequireRole("admin")` (or `RequireMinLevel(50)`). Add a new `RequireRole(slug)` / `RequireAnyRole(slug...)` helper. Update every call site in `internal/guards/`, `internal/pocketbase/routes/`, `internal/websocket/handlers/`, etc. The frontend mirror in [sveltekit/src/lib/utils/guards.ts](sveltekit/src/lib/utils/guards.ts) likewise switches from `isAdmin` boolean to a roles-array check; [auth.svelte.ts](sveltekit/src/lib/stores/auth.svelte.ts) hydrates `roles: string[]` from `/api/me` instead of `isAdmin: boolean`.

### 8d. Self-service + admin UI

Admin page at [sveltekit/src/routes/admin/roles/](sveltekit/src/routes/admin/roles/) for managing role definitions and assignments. Users see their own roles on their profile page (M15 will surface this).

Smoke test: log in as a pre-migration admin → user record now shows `roles: ["admin"]`, `isAdmin` field gone; admin UI works exactly as before. Create a `tournament_organizer` role, grant to a non-admin user → confirm M16 tournament-create routes accept the request when wired (or at least confirm the guard plumbing accepts the role check).

## Milestone 9 — Match-aware kiosk view

> A logged-in player should see the kiosk for the container they're playing in (and only that one), automatically. The existing per-container kiosk at [sveltekit/src/routes/containers/[name]/+page.svelte](sveltekit/src/routes/containers/[name]/+page.svelte) is admin-gated and assumes you know the container name. Replace that mental model with "log in → see your match", driven by gamertag-to-machine-name detection inside the running scraper data.

### 9a. Gamertag → machine-name detection

Each scraper runner already exposes the local Xbox machine name and the network player roster (machines + gamertags) via the M5 `instanceCache`. Extend the runner to publish a `(container, machines[], gamertags[])` membership view — likely a new field on the `host:all` summary aggregator in [internal/scraper/manager/aggregator.go](internal/scraper/manager/aggregator.go). The same view feeds M10's overlay routing.

### 9b. Per-user "my match" page

New route at `/play/` (or `/my-match/`). Subscribes to `host:all`, finds the container whose roster contains any of the logged-in user's gamertags (from M7c), then renders the existing kiosk iframe + controller UI for that container. Renders blank/idle state otherwise. Auto-refreshes if the user's gamertag appears on a different container later.

### 9c. WS auth narrowing

Today `host:<name>` requires admin. Extend the room guard so a non-admin user is permitted to join `host:<name>` if they have a gamertag in that container's roster. New room-level guard in [internal/websocket/rooms/host.go](internal/websocket/rooms/host.go) or a new sibling guard, consuming the new role helpers from M8c so admins always get in regardless of roster membership. Keep `host:all` admin-only (it's a cross-instance summary).

Smoke test: 4-container match, 4 logged-in users with one gamertag each, each gamertag mapped to one container's local Xbox machine. Each user opens `/play/` → sees only their container's kiosk. Admin opens `/play/` while not playing → blank state, but admin can still hit `/containers/<name>/` directly. User logs in but isn't in any active match → blank state.

## Milestone 10 — Overlay revamp + new browser sources

> Current overlay at [sveltekit/src/routes/overlays/players/+page.svelte](sveltekit/src/routes/overlays/players/+page.svelte) is keyed to `firstGameData` / `firstTick` (the legacy single-instance accessor) and shows local players only. Rebuild around the M5 multi-instance model where overlays bind to a specific machine's POV — sometimes the host container, sometimes a guest the host is connected to — and add new overlay surfaces beyond the current player HUD.

### 10a. POV-bound overlay routing

Route shape: `/overlays/<surface>/<machine_name>/` (surface first — groups by overlay type, e.g. `/overlays/scoreboard-detailed/halo-host-1/`). The overlay subscribes to the `host:<container>` room whose roster contains `<machine_name>` (lookup via the M9a aggregator extension). Players' POV is then rendered relative to that machine's seat in the local players list. Replace `firstGameData` / `firstTick` with this lookup pattern; deprecate the legacy accessors.

### 10b. Scoreboard surfaces

Two browser sources at `/overlays/scoreboard-simple/<machine>/` and `/overlays/scoreboard-detailed/<machine>/`. Simple = team scores + match clock. Detailed = full per-player K/D/A, current weapons, alive/dead state.

### 10c. Event popup overlay

`/overlays/events/<machine>/`. Renders animated card-style popups for kill chains (multi-kills, kill streaks), CTF captures, oddball/hill events, juggernaut transitions. Likely needs an animation library beyond raw CSS — candidates: Svelte's built-in `transition` + `motion`, or a small library like `@svelte-motion`. Decide during 10c.

### 10d. Dummy-player / neutral-host filter

In modded Halo: CE matches with a neutral host, the host container spawns a dummy player out-of-bounds that never participates. Without filtering it shows up in the overlay, the scoreboard, and (later) the stats. Implement a filter at the data layer in [internal/scraper/manager/](internal/scraper/manager/) (or a sibling helper) so the same filter applies to overlays, minimaps (M11), and stats (M15). Three configuration sources:

- Per-container flag `is_neutral_host` (likely added to the container record managed by [internal/podman/](internal/podman/) or as a sidecar config; defaults false).
- A global allowlist of "always-dummy" gamertags (configurable via PB schema in 10d, e.g. `dummy_gamertags` collection).
- A per-game manual override accessible from the M15 stats UI for after-the-fact correction.

The filter takes a roster + the container's neutral-host flag and returns the cleaned roster. Overlays/minimaps consume the cleaned roster; raw debug page (M6c) still shows the unfiltered view for diagnostics.

### 10e. POV correctness pass

Today the overlay assumes the rendering machine *is* the local one. After 10a's refactor, the overlay can be POV-bound to any machine in any container's roster — confirm tag names, weapon slots, and stat indices are correct for the targeted machine, not the host. Likely surfaces edge cases in the Halo: CE reader; file follow-ups for M19 if found.

Smoke test (matches M5's 4-instance pattern): start 4 containers (one flagged neutral-host) in a system-link match, open `/overlays/scoreboard-detailed/<machine_a>/` and `/overlays/events/<machine_b>/` in separate OBS Browser Sources, run a 5-minute match. Verify POV correctness, animation timing, OBS transparency, and that the neutral-host's dummy player is absent from both overlays. Re-validate the existing players overlay through the new routing.

## Milestone 11 — Game minimaps

> Browser-rendered minimap as another overlay surface. Show floor outline, player positions + view direction, power-weapon / power-up spawns, height differential cues, animated event flares, and (if feasible) projectile traces. Extends M10's overlay infrastructure but warrants its own milestone because of the rendering complexity.

### 11a. Map geometry feasibility

Audit what the Halo: CE scraper actually exposes for level geometry. Today the reader carries `power_item_spawns` and `map` identity but probably not BSP geometry. Decide between: (i) baking per-map static SVG/PNG tracings into the frontend keyed by map name + scenario tag, (ii) extracting BSP at runtime from the scraper, (iii) hybrid (static floor, dynamic markers). Almost certainly (i) for v1 — floor outlines as committed assets in `sveltekit/static/maps/<scenario>.svg`. For multi-floor maps, commit per-floor variants and use Z-coordinate ranges (see 11c) to switch between them.

### 11b. Player position + view cone

The reader exposes player world coordinates and aim vectors per the M5 tick payload. Project onto the 2D minimap via a per-map transform (committed alongside the SVG asset in 11a). Render with HTML5 canvas or SVG primitives. Filter the roster through M10d's dummy-player filter so the host's out-of-map dummy doesn't show up.

### 11c. Height differential cues

Map a player's Z (vertical) world coordinate to a visual cue on the 2D minimap so viewers can tell who's elevated vs underneath. Two complementary approaches, ship both and let style toggle pick:

- **Icon size scaling** — closer to the camera (higher Z, or whatever convention fits the map) = larger icon. Subtle range, e.g. 0.7×–1.3×.
- **Color tint / Z-banded layer** — segment Z into N bands and tint the icon (e.g. blue tint = below floor, red tint = above floor). Good for sharp multi-floor maps where size scaling reads ambiguously.

### 11d. Power weapons + power-ups

`power_item_spawns` already in tick data; render as fixed icons. Add held-or-available state if the reader exposes it; otherwise that's a follow-up offset to add (file under M19).

### 11e. Event flares + animations

When certain events fire on a tick, animate a flare on the minimap at the relevant position. Initial event list (each gets its own animation):

- **Death + respawn** — fade-out at death position, brief flash at respawn.
- **Player teleporting** — line/streak between source and destination teleporter exits.
- **Active overshield / camo** — persistent halo or shimmer around the icon while the powerup is active.
- **Power weapon held** — small badge on the icon (rocket, sniper, etc.).
- **Multi-kill / kill streak** — burst flare at the killer's position.

Animation library decision (see 11g) gates how rich these can be; v1 can ship CSS-keyframe-based animations and upgrade later.

### 11f. Projectile rendering (stretch)

Investigate whether the projectile data the reader currently exposes (visible in the debug page Tick → Projectiles tab) is rich enough for tracer rendering. If not, spec the offset additions and file as M19 follow-ups; ship 11a-e without projectiles.

### 11g. Library choice decision

Raw canvas vs. animation library (PixiJS, two.js, Konva, motion-one). Decide during 11b/11c based on perf — 30Hz tick updates × 16 players (with size + tint deltas) × N projectiles + N flares may justify a real renderer. SVG with Svelte transitions might be enough for 11a-d; flares (11e) and projectiles (11f) probably push toward canvas.

Smoke test: load `/overlays/minimap/<machine>/` for a Halo: CE match on Blood Gulch (or whatever map's been traced first). Player positions track correctly, view cones rotate with aim, height cues swap correctly when a player jumps a cliff, power weapons appear at spawn positions, kill flares fire on every kill captured by the M5 event stream, neutral-host dummy player is absent. Composite over OBS scene.

## Milestone 12 — POV marker overlay (stretch)

> Long-shot. Browser source that overlays directly on top of a machine's actual game POV (e.g. as an OBS Browser Source layered above the kiosk capture), drawing world-anchored markers in real time: enemy silhouettes through walls, powerup tags, teammate gamertags floating above their heads. Requires perspective projection (3D world → 2D screen) instead of M11's top-down projection — same input data, harder math.

### 12a. Camera + projection model

Halo: CE per-player camera state (FOV, position, view direction) is already partially read by the scraper. Audit what's there; spec any missing offsets (camera matrix, near/far planes) for an M19 follow-up if needed. Build a per-tick "render frustum" model in the overlay client.

### 12b. Single-player POV alignment

New overlay at `/overlays/pov/<machine>/`. For full-screen single-player on the target machine: project enemy world positions through the player's camera matrix to screen coordinates. Render markers (silhouettes, name tags, distance indicators) as positioned absolute elements. Validate alignment by overlaying onto the actual kiosk video stream — markers should track tightly as the player turns.

### 12c. Split-screen handling

When the target machine is running 2/3/4-player split-screen, partition the screen into the appropriate viewports and project per-viewport per-player. The M5 reader already exposes local-player count + indices.

### 12d. Marker types

Initial set:

- Enemy silhouettes (filled shape outlining the enemy's bounding box, with team color).
- Teammate gamertag floats above-head.
- Powerup labels at spawn positions (with active/respawn timer if available).
- Optional: line-to-objective in CTF (toward the enemy flag if you're attacking, toward your base if you have it).

### 12e. Composite verification

OBS scene = kiosk video source layered with the POV overlay browser source above it. Test rig: known-good viewing angle on a known map → measure pixel offset between marker and actual entity at multiple angles. If offsets are consistent, calibration matrix is correct; if they drift, the camera offsets are off and feed back to 12a as offset bugs.

Smoke test: 1v1 Slayer on Wizard, single full-screen → enemy silhouette tracks the opponent through walls; teammate-tag-above-head test in a 2v2 game on Hang 'Em High. Split-screen verification: same setup with 2v2 on a single console.

**Stretch flag.** This milestone is explicitly stretch — if M11 reveals that the projection math is brittle, defer M12 to M21+ open bucket and ship M11 alone. Also explicitly out of scope for v1: through-wall occlusion (rendering markers dimmed when behind geometry), since that requires BSP knowledge from M11a's deferred case.

## Milestone 13 — PocketBase persistence foundation: games + series

> Replaces the original M6 "port snapshots/events/sessions/overlay_state" framing. The new data model is **game** (singular contest) + **series** (a group of one or more games with a format and a category). Pickup matches are 1-game series; tournament rounds are best-of-N or first-to-X series. Categorization is a field on series, defaulted by a gametype-variant-name heuristic and editable after the fact.
>
> **Terminology:** "game" = singular contest (one round of Slayer, one CTF round). "Series" = a grouping of one or more games with a format and category.

### 13a. Schema design

Collections under [internal/pocketbase/schema/](internal/pocketbase/schema/), one file each:

- `series` — `{name?, format: "exact-N"|"first-to-X", target_n, category: enum, created_by, started_at, ended_at?, tournament?, tournament_round?}`. Category enum at minimum: `casual | competitive | tournament | custom`.
- `games` — `{series, container, host_machine_name, map, gametype, variant_name, started_at, ended_at, winner_team?, score_summary, snapshot_blob?}`.
- `game_events` — `{game, tick, type, payload}`. Append-only event log.
- `game_players` — `{game, gamertag, team, kills, deaths, assists, score, time_alive_ms, weapon_loadout?}`. One row per player per game.

Decisions to lock during 13a: snapshot blob format (full instanceCache JSON vs trimmed?), retention policy on `game_events` (full forever vs roll up to `game_players` + drop after N days?), how series surface "in progress" state (an absence of `ended_at` plus a join-table to active games?).

### 13b. Game-end persistence wiring

Hook into M5 manager's Live → Ready transition (the path that already populates `cache.PreviousGame`). Write a `games` row + N `game_players` + the event stream. If no `series` exists, create a 1-game `casual` series automatically.

### 13c. Variant-name → category heuristic

Build a small lookup table mapping variant-name patterns (regex or substring match) to suggested categories — `Slayer`/`CTF` (default Halo variants) → `casual`; `Tournament` / `MLG` / `Comp` substring → `competitive`; explicit override flag from the M14 series setup beats the heuristic. Heuristic populates the suggested category on game creation; admin can re-categorize anytime through PB admin UI.

### 13d. Replace silent-drop queue (legacy bug)

Port `internal/pb/client.go` queue from legacy with one of:

- (a) Retry with exponential backoff.
- (b) Disk-spool overflow.

Decide during port; comment the tradeoff.

Smoke test: run a Halo: CE Slayer game start-to-finish on a single container → one `series` (category `casual`) + one `games` row + N `game_players` rows + event stream all land. Re-run with variant name "MLG Tournament v7" → category auto-suggested as `competitive`.

## Milestone 14 — Series management: setup, pick/ban, in-progress UI

> M13 lets the system *record* games as they happen; M14 lets users *intentionally set up* a multi-game series before play, optionally with a pick/ban round, and display the series in progress. Pick/ban is opt-in: if a series is set up with pick/ban, maps are committed up-front; otherwise the series just records whatever's played, game by game.

### 14a. Series creation UI

New page at `/series/new/`. Pick format (single, exact-N, best-of-N, first-to-X), participants (one or more teams from M7, or ad-hoc gamertags), category override, optional name. Creates a `series` row in the not-started state.

### 14b. Pick/ban round (optional)

When series creator opts in: present a draft-style map list, alternate ban / pick between participating teams, store the resulting map order on the series. UI flow can run synchronously in the browser or async via PB realtime — decide during 14b.

### 14c. Series-in-progress UI

New page at `/series/[id]/`. Shows series header (format, participants, category), per-game scoreboard (one row per played game), current standing (X-Y in a best-of-5), next map (if pick/ban committed) or "TBD". Auto-updates via PB realtime as new `games` rows are written by M13b.

### 14d. Series-aware game-end wiring

Extend M13b: when a game finishes and the host container is running under an active series (matched by container or gamertag), attach the new `games` row to that series instead of auto-creating a casual one. Series ends when format completion is reached (e.g. one team has won 3 of 5).

Smoke test: create best-of-3 series with 2 teams + pick/ban → 3 maps committed. Play 2 games (one team wins both). Series UI shows 2-0, marks series complete, doesn't accept the 3rd map. Compare to a casual no-pick/ban series of "first-to-2": same termination behavior driven by the format field.

## Milestone 15 — Per-user / per-team stats

> Aggregate stats computed from M13's `games` + `game_players` data. Per-user aggregation rolls across the user's gamertags (from M7); per-team aggregation rolls across team trosters.

- **15a. Stats query layer.** Internal helpers (Go-side or PB hooks) for per-gamertag, per-user (all gamertags), per-team aggregations: K/D, W/L, win rate, time played, per-game-type splits.
- **15b. Stats UI.** Profile page at `/u/[username]/` showing stats with filters (game type, category, date range, per-gamertag breakdown). Team page at `/teams/[slug]/` mirroring the same.
- **15c. Match-history view.** Recent games list with links to series + game detail pages. Shareable URLs.
- **15d. Dummy-player override.** UI to mark a `game_players` row as "dummy / neutral host" after the fact, excluding it from aggregates. Reuses the M10d filter at the data layer.

Smoke test: play 5 games across 2 gamertags belonging to the same user → profile page shows correct rolled-up K/D and per-gamertag breakdown; per-game-type filter works; team stats include only games where players were repping that team; a manually-flagged dummy-player row is excluded from aggregates and the match-history view.

## Milestone 16 — Tournament system

> Tournaments group multiple series with structure (bracket or round-robin). Each tournament round is one series from M14; matches are the games within those series. Bracket rendering on the site.

- **16a. Schema.** `tournaments` `{name, slug, format: "single-elim"|"double-elim"|"round-robin"|"swiss", participants, started_at, ended_at?}`, `tournament_rounds` `{tournament, round_number, series_a, series_b?, winner_advances_to?}`. Series records gain optional FK back. Tournament create gated by `tournament_organizer` role from M8.
- **16b. Bracket / round-robin generators.** Create a tournament → auto-generate the round structure based on participant count + format.
- **16c. Tournament UI.** `/tournaments/[slug]/` rendering bracket or round-robin grid. Live updates as series complete. Click into any round → M14's series-in-progress UI.
- **16d. Tournament-aware series creation.** Inside a tournament, spawning the next round creates a series pre-tagged with `category: tournament` + `tournament + tournament_round` FKs.

Smoke test: 8-team single-elimination → bracket renders, play through round 1 → 4 series + 4+ games persist, bracket auto-advances winners, round 2 series spawn correctly. Repeat with 4-team round-robin. Confirm a non-`tournament_organizer` user gets 403 on `POST /tournaments`.

## Milestone 17 — Discord integration: stats lookup + per-guild channel posting

> Bot already exists ([internal/disgo/](internal/disgo/)) but does little besides the placeholder `ping`. This milestone makes it useful: stats commands, automatic posting of game/series/tournament events to configured guild channels.

- **17a. Per-guild config schema.** `discord_guilds` `{guild_id, results_channel?, tournament_channel?, posted_categories: enum-list}`. Admin UI for guild owners to configure (likely a slash command `/cartographer config` rather than a web UI to start).
- **17b. Stats slash commands.** `/stats user:<gamertag>`, `/stats team:<slug>`, `/recent user:<gamertag>` — consume M15 stats helpers, render as Discord embeds via [internal/disgo/components/](internal/disgo/components/).
- **17c. Event posting.** When a game/series/tournament-round finishes, post an embed to each guild's configured channel (filtered by `posted_categories`). Use the existing `routine.FireAndForget` pattern from PB hooks (CLAUDE.md convention).
- **17d. Tournament announcements.** Bracket-update embed when a tournament round completes; new-tournament announcement on creation.

Subsumes the basic ops commands originally in old M8 (session start/stop, who's-playing-now) — fold those into a single `/cartographer` command group rather than top-level slash commands.

Smoke test: run a tournament series with auto-posting to a test guild — game results post within seconds; `/stats` returns correct data; per-guild category filter prevents casual games from spamming a tournament-only channel.

## Milestone 18 — Rating system + multiple leaderboards

> Build a per-game-type rating on top of M15's stats and surface leaderboards on the site + via Discord.

- **18a. Rating algorithm choice.** Decide between TrueSkill, Glicko-2, ELO, or a simpler K-factor system. Trade-offs: TrueSkill handles team/FFA out of the box; Glicko-2 has uncertainty bands; ELO is simplest. Document choice + why in code.
- **18b. Per-game-type rating.** A player has a separate rating per game type (Slayer rating ≠ CTF rating ≠ Oddball rating). Recompute on every game finish via a PB hook on `games` insert.
- **18c. Leaderboard surfaces.** `/leaderboards/<type>/` — game-type, category (only-competitive, only-tournament, all), and time-window (all-time, season, last-30-days) facets. Default landing at `/leaderboards/`.
- **18d. Discord leaderboard commands.** `/leaderboard type:<game> [category:<cat>]` → top-N embed. `/rank user:<gamertag>` → user's current rating + rank. Re-uses M17 plumbing.

Smoke test: play 30 games across 3 game types and 2 categories → ratings update each game-end; leaderboard pages render with correct sort and filtering; Discord commands match the web view.

## Milestone 19 — Robustness + offset validation

> Split from the original M8 (Robustness + Discord + auth). The Discord pieces (commands, channel posting) are subsumed by M17; the auth-wrapping work is mostly addressed by M6a + M7c + M8 + M9c; multi-user UX is covered by M15/M16. What remains is the "make silent bad data become loud errors" work, plus general operational hardening.

- **19a. Runtime offset sanity checks.** Apply to both Halo: CE and Halo 2 (whenever it lands). Base-HVA range checks, magic-value probes, plausibility bounds on read values. Loud error (log + WS notification + admin debug page badge) on sanity-check fail.
- **19b. Field-level validation.** Use the M6c debug-page audit's "looks plausible / clearly broken" annotations as input — promote validated fields out of `unverified` status, file remaining `unverified` fields as offset-investigation tasks.
- **19c. PB queue + scraper resilience.** Polish M13d's queue logic; add metrics; ensure scraper restarts cleanly after PB outages.

Smoke test: deliberately corrupt an offset in the Halo: CE table → loud error fires within one read; recover by reverting; confirm no silent bad-data writes to PB.

## Milestone 20 — Halo 2 scraper (with known caveats)

> Demoted to last per user direction. Validates the registry abstraction holds for a non-CE Halo title, and consumes M19's offset validation from day one.

- Port `internal/scraper/halo2/*` preserving **every** `UNVERIFIED` comment.
- Known broken areas (each gets its own follow-up task):
  - Event buffer (`GVAEventCount` always reads 0) — may not exist in xemu's layout; re-derive offsets or find an alternative data source.
  - Objects datum array → real `Alive / Health / Shields / Vehicle` values (currently hardcoded stubs).
  - Team index / primary color / gametype (`SessOffTeamIndex`, `SessOffPrimaryColor`, `GRGVarGameTypeOff`).
- Wire into M19 offset validation — H2 fields enter the system already gated by sanity checks.
- Wire into M13/M15/M16/M18 — game records, stats, tournaments, ratings all become game-type aware including Halo 2.

## Milestone 21+ — Open

- **qcow2 disk-image editing.** Long shot. Easiest path may be running an FTP server inside the xemu container so UnleashX's FTP client can drop files onto the guest disk. Alternative: mount qcow2 host-side via libguestfs / `qemu-nbd` and edit FATX directly. Investigate before committing to either.
- **POV marker overlay (M12 fallback target).** If M12 turns out infeasible (perspective math too brittle, camera offsets unreliable), demote it here as a research item rather than a committed milestone.
- Second-game generalization test (confirm the scraper registry abstraction holds for something non-Halo) — partially answered by M20 but stays open for a third title.
- Community-contributed offset tables (moderation workflow).
- Post-game report UI (replaces HaloCaster's Excel export) — could land alongside M15 stats as a follow-up.
- Hosted / remote deployment story.

---

## Explicit non-goals (for now)

- Desktop GUI (WinForms, DearPyGui) — web is the UI.
- `cmd/{memscan,prove,localproof}` offset-discovery tools — re-derive on demand.
- Halo-specific logic leaking into `internal/xemu/` or the top-level `internal/scraper/` — domain code stays in `internal/scraper/<game>/`.

## Open questions to pin during M2–M13

- **WebSocket format:** adapt legacy `Envelope` to the template's `message.Message`, or extend the template's schema? Decide in M2.
- **PocketBase overload policy:** retry-with-backoff vs. disk-spool. Decide in M13.
- **Podman privilege model:** legacy requires root Podman (KVM + DRI + NET_ADMIN). Keep the requirement or explore rootless (would lose direct device access)? Decide in M3.
- **Deployment model:** same-host (server + xemu on one machine, matches legacy) vs. distributed (thin memory-reader agent + remote PocketBase). Default same-host unless blocked.
