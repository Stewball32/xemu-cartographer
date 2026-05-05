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

- **2a — full offset audit.** Reconciled all 515 hex constants from `atlas/HaloCaster/HaloCE/halocaster.py` against the 128-offset legacy Go table. Active read-path constants live in [internal/scraper/haloce/offsets.go](internal/scraper/haloce/offsets.go); every other corroborated offset organised by struct in [internal/scraper/haloce/offsets_reference.go](internal/scraper/haloce/offsets_reference.go). Each constant carries a `// halocaster.py:NNN` origin tag. All marked `unverified` until M8's runtime sanity-check pass.
- **2b — scraper code ported.** [reader.go](internal/scraper/haloce/reader.go), [events.go](internal/scraper/haloce/events.go) (19 event types via stat-diff + damage-table fallback), [game.go](internal/scraper/haloce/game.go) (`init()` registers Halo: CE with `scraper.Lookup`), [xboxname.go](internal/scraper/haloce/xboxname.go).
- **2c — WS wiring.** New [internal/scraper/manager](internal/scraper/manager) package owns per-instance lifecycle (Start / Stop / List) and the 30Hz tick goroutine. Decision: **wrap, not extend** — every broadcast becomes `Message{Type:"scraper", Room:"overlay", Payload:<envelope-json>}` so the wire schema stays uniform across all rooms ([loop.go](internal/scraper/manager/loop.go)). New `overlay` room with `RequireAuth` ([rooms/overlay.go](internal/websocket/rooms/overlay.go)). New `Scraper` field on `guards.Services` backed by `internal/guards/interfaces/scraper/` (one-method-per-file).
- **2d — admin routes + main.go wiring.** `GET /api/admin/scraper`, `POST /api/admin/scraper/start`, `POST /api/admin/scraper/stop/{name}` ([routes/scraper](internal/pocketbase/routes/scraper)), all gated by `RequireAuth + RequireAdmin`. `cmd/server/main.go` builds the `Services` skeleton early so the scraper manager gets a stable `*Services` pointer; subsystems mutate fields as they come up. Blank import `_ "internal/scraper/haloce"` triggers the title-ID registration.

### M2 follow-ups (deferred)

- ~~**Snapshot replay for late joiners.**~~ Resolved during M4 with option (a): each `runner` caches the most-recent `websocket.Message` bytes for its snapshot envelope; the `join_room` handler replays them via the new `SendRaw` event capability when a client subscribes to `overlay`. See `internal/scraper/manager/{runner.go,loop.go,manager.go}`, `internal/guards/interfaces/scraper/snapshot.go`, `internal/websocket/handlers/join_room.go`.
- **Investigate `power_items: null` in tick payloads.** During the smoke test the initial snapshot's `PowerItemSpawns` came back empty (likely the scenario wasn't fully loaded when the scraper started, since power-item resolution depends on world-object scanning). Worth re-running the smoke test with start-after-match-ready and confirming spawns populate; if they still don't, that's a Halo offset divergence to chase during M8.

### 2a. Offset audit (prerequisite)

The legacy Go offset table has 128 hex constants; HaloCaster's `HaloCE/halocaster.py` has 515 scattered across 2587 lines. Before trusting the legacy table as complete:

1. Read `atlas/HaloCaster/HaloCE/halocaster.py` end-to-end, extracting every memory-offset-like constant with surrounding context (what struct, what field, what read type).
2. Diff the extracted set against `atlas/xemu-cartographer-legacy/internal/scraper/haloce/offsets.go`.
3. Categorize the deltas:
   - Genuinely missing offsets the legacy reader never used → port them.
   - Non-offsets (struct sizes, magic values, indexing math) → document in comments, don't port.
   - Offsets that exist in both but differ in value → investigate (xemu vs. real-Xbox divergence is plausible).
4. Produce a reconciled `internal/scraper/haloce/offsets.go` in the new repo, each offset annotated with its HaloCaster origin (file + line) and verification status.
5. Flag offsets needing runtime verification for Milestone 8's sanity-check work.

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

Restructure the scraper pipeline and the WS emission layer around a clear three-phase model (Idle / Ready / Live) with an authoritative per-instance cache as the source of truth. The current implementation works but conflates lifecycle, scrape cadence, broadcast wrapping, and *caching of pre-marshaled wire bytes* in the runner; clients reconstruct state from a stream of envelopes rather than reading a coherent cached object on connect. This milestone introduces explicit phases, a structured cache (`instanceCache`), a per-instance room (`host:<name>`) + aggregate room (`host:all`), and a cleaner emission protocol that *builds* envelopes from the cache on demand instead of replaying stale broadcast bytes.

Driving brief: [scraper-ws-refactor-brief.md](scraper-ws-refactor-brief.md).

**Honors existing conventions:** one-per-file registration, `internal/scraper/manager` and `internal/websocket/handlers` package layouts, `guards.Services` for cross-system access, and the M2c wrap-not-extend envelope policy (`Message{Type:"scraper", Room, Payload:<envelope>}`).

### Terminology

The word "snapshot" is **deliberately retired** by this refactor. Today it appears in three different roles in the codebase, and conflating them is part of why the wire model is hard to reason about:

- **Legacy: `snapshot` as an envelope type.** Today the runner broadcasts envelopes of type `"snapshot"`, `"tick"`, and `"event"`. After this milestone, those types are replaced by `current_state` (full cache contents) + `state_update` (per-scrape cadence cache update) + `event` (unchanged in spirit). The new wire protocol contains no envelope named "snapshot".
- **Legacy: `ReadSnapshot()` as a `GameReader` method name.** Today this method reads a mix of static (map, gametype, scenario data) and volatile (roster scores, team scores) fields. The method may keep the name for diff-readability or be renamed (see open question 3) — but the name is *internal* and not meaningful to the wire protocol.
- **Legacy: `runner.latestSnapshotMsg` and `Manager.LatestSnapshotMessages()`.** These cache the marshaled `Message` bytes for the most recent `snapshot` envelope and replay them on `join_room`. Both are removed in 5a (cache becomes structured) and 5c (replay becomes a `current_state` build from the cache).
- **Generic English usage.** When a brief or doc says "atomic snapshot of state", the intended meaning is "an atomically-read consistent view of the cache". This milestone uses **"atomic cache read"** instead.

After this milestone, references to "snapshot" in code, comments, log lines, and docs should either disappear or be qualified by which legacy role they refer to. New names:

| Concept (new model)                                        | Term used here                            |
| ---------------------------------------------------------- | ----------------------------------------- |
| Full cache contents emitted on join + phase transition     | **`current_state` envelope**              |
| Per-scrape cadence update of the tick-fields portion       | **`state_update` envelope**               |
| Discrete happenings during Live (kills, pickups, etc.)     | **`event` envelope**                      |
| The structured per-instance cache held by the runner       | **`instanceCache`**                       |
| The aggregated cross-instance summary cache                | **`hostsCache`** (drives `host:all` room) |
| An atomically-read consistent view of `instanceCache`      | **"atomic cache read"**                   |
| Live-match fields fixed for the duration of the match      | **"match-static fields"**                 |
| Live fields that change during play                        | **"tick fields"**                         |

### Out of scope

- **PocketBase persistence** of events / final match state — leaves the in-memory `previous_game` shape such that a future flush is straightforward (M6, formerly M5).
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
- **XBE-swap correctness fix.** `xemu.Instance.LowHVA` returns a cached HVA from the one-shot `Init`-time GVA→GPA→HVA translation. Across XBE swaps (dashboard → game, game → dashboard, game → game) the kernel keeps the guest VA but moves the underlying physical page, so the cached HVA reads stale bytes from the previous mapping — which would have made every Idle poll return the *first* XBE's title ID forever. New `xemu.Instance.RefreshLowHVA(gva)` re-runs the QMP translation in place, and `scraper.ReadTitleID` now always re-translates so the Idle / Ready title-ID polls observe XBE swaps correctly. Same fix means the OQ6 probe captures fresh data on every sample rather than re-reading the start-time HVA.
- **Ready stuck-on-errors fix.** The Ready loop now runs the title-ID re-check *before* `ReadGameState`, so when an XBE swap leaves Halo's reader pointing at stale / unmapped addresses (and `ReadGameState` returns errors every iteration), the runner still escapes to Idle within ~5s rather than looping forever on the failed read path.

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

**Why first:** every other stage depends on a structured cache existing. Locking down the phase machine and the cache shape *before* changing the wire keeps the diff readable and lets us verify the in-memory model in isolation against a live xemu.

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

- Update [internal/websocket/handlers/request_state.go](internal/websocket/handlers/request_state.go) to look up the `host:*` room the requester is in and reply with the new `current_state` envelope for that room (via `e.SendRaw` so it's addressed only to the sender). Today's handler returns legacy snapshot bytes for all instances — narrow it to a single `current_state` build for the requester's room.
- Add `request_events` handler. Optional filters: `since_tick` (events with `tick > N`, for resync after a connection gap) and `types` (string-array filter). With no filters, return the full Live-phase event log. **In Idle and Ready, return an empty list even when `previous_game` exists** (resolves open question 1). Reply via `e.SendRaw`.
- Document the ordering convention for `request_events`: events returned in the same order they were appended to the live event stream (open question 7).

**Why fourth:** depends on (5c) for envelope shapes; the new `request_state` reply has to use the new `current_state` shape.

**Defers:** any persistence-backed event lookup (out of scope; in-memory only).

### 5e. SvelteKit client update

- Replace single hardcoded `join_room` to `"overlay"` with per-instance subscription: pages join `host:<instance>` based on the route param, the admin debug page can subscribe to multiple, the instance-picker UI subscribes to `host:all`.
- Replace today's legacy `Envelope = {snapshot|tick|event}` consumer with the new envelope set: `current_state`, `state_update`, `event`.
- Wire `request_state` and `request_events` into the store for resync after a connection gap.
- Update [scraper-ws.svelte.ts](sveltekit/src/lib/stores/scraper-ws.svelte.ts), the type definitions in [scraper.ts](sveltekit/src/lib/types/scraper.ts), the players overlay, and the admin debug page tabs.

**Why last:** all backend wire-format work must land first; the frontend churn is contained here and ships in lockstep with the new protocol.

**Defers:** persistence-backed history views (M6, formerly M5).

### Open questions (with proposed resolutions)

1. **`request_events` outside Live** — *Resolved by brief.* Returns empty in Idle and Ready, even when `previous_game` exists. Rationale: a client asking for events shouldn't have to also check phase to know whether the response is "live" or "from a finished match". If post-game replay becomes useful later, expose via a separate `request_previous_game` message.

2. **Default-room update granularity** — *Proposed: full list re-broadcast.* Aligns with the brief's recommendation. Payload is small (a handful of summary records); diff logic is more complex and won't pay off until we have many more instances. Revisit if the per-instance summary grows or the host count grows large.

3. **`GameReader` interface evaluation** — *Proposed: minimal extension + rename.* Reading [internal/scraper/haloce/reader.go](internal/scraper/haloce/reader.go) shows the existing methods map cleanly: `ReadSnapshot` already caches scenario-static data and re-reads volatile fields on each call (matches Live's static-fields-cached + Ready's full reread); `ReadLobby` is the explicit cheap variant for non-in_game (matches Ready's cadence); `ReadTick` matches Live's tick reads. The only misfit is **Idle**, which today reads the ambient game state via `ReadGameState()` but has no notion of "Xbox machine name + freshness indicator". Proposed interface changes:
   - **Add `ReadIdleData() (IdlePayload, error)`** returning `{title?, machine_name, clock_or_freshness_value, last_read_at}`.
   - **Rename `ReadSnapshot` → `ReadMatchState`** (or `ReadFullState`) to retire the overloaded "snapshot" term. The method name is internal — no wire impact — and the rename clarifies that it reads the match-data field set (static + volatile), distinct from the wire-protocol `current_state` envelope. Consistent with the brief's explicit retirement of the "snapshot" term.
   - **Rename `ReadLobby` → `ReadReadyState`** (or `ReadActiveState`) for the same reason — "lobby" is one of several Ready-phase contexts (lobby, post-match stat screen, between-match menu).
   - Keep `ReadTick`, `ReadGameState`, `OnStateChange`, `BuildScoreProbe`, `LastStateInputs`, `NewTickState`, `XboxName`, `Title`, `LowGVAs` unchanged.
   - Do **not** predeclare a separate static/tick method split on the interface — the existing `ReadSnapshot`/`ReadTick` pair plus internal scenario caching in the plugin (today's pattern) already deliver the right behavior.

   Renames are mechanical and can land in 5a alongside the cache work, or as a prep-stage 5a-prelude commit, depending on how clean the diff needs to be.

4. **Idle-phase scraping mechanics** — *Proposed: single runner per instance lifetime with a hot-swappable reader.* Today `Manager.Start` fails when `scraper.Detect` doesn't recognize the title — no runner is created and the discovery watcher logs the failure. New model: `Manager.Start` always creates a runner; the runner owns the `*xemu.Instance` for the whole socket lifetime. The runner enters Idle with no `GameReader`. On title-ID becoming recognized, the runner loads the matching reader (registry lookup) and transitions to Ready. On title-ID becoming unrecognized (Live → Idle or Ready → Idle), the runner drops the reader and returns to Idle. Justification: keeps lifecycle tied to socket presence (matches discovery's mental model), avoids the complexity of two runner classes and handoff between them, and leaves a clean place for the Idle-phase poll loop.

5. **Reserved-name enforcement for `all`** — *Proposed: confirm chokepoint approach.* Single function `roomForInstance(name) (string, error)` in [internal/websocket/rooms/](internal/websocket/rooms/) is the only sanctioned way to derive a room name from an instance name. Returns error on `name == "all"` (or anything that contains `:` or other reserved characters). Every code path that needs a room name goes through it. PocketBase API rules and podman create-validation can layer on top for user-facing rejection at create time, but the chokepoint is the trust boundary. The discovery watcher in [internal/discovery/](internal/discovery/) needs a small change to filter out `.sock` files whose stem is `all` so the chokepoint never sees that name from disk.

6. **Load-out detection gap** — *Proposed: investigate during 5a, two candidate causes.* The current [loop.go:168-181](internal/scraper/manager/loop.go#L168-L181) periodic check (every ~5s during idle states only) compares `scraper.ReadTitleID(r.inst)` against the start-time `r.titleID`. Two candidate causes for the Halo CE → dashboard miss:
   - (a) The XBE header at GVA `0x00010000` retains the old title ID after game exit because xemu doesn't re-load that page when the dashboard takes over.
   - (b) The check only runs in idle states; if the runner is in-game when the user quits, no Live → Ready transition fires (the game can't emit it if it's gone).

   Investigation plan: add a debug probe that continuously reads the title ID + a dashboard-detection heuristic (e.g., presence of an XBE-magic check, a known dashboard title ID, or a memory-region nullity test) across a real Halo CE → quit-to-dashboard transition; pick the most reliable signal. If the title-ID address is the right place but the read is stale, propose re-translating the GVA on each check (xemu may have remapped the page). If the title-ID address is unreliable, fall back to a state-machine signal (e.g., heartbeat: if `ReadGameState` errors for N consecutive polls, assume Live → Idle). Document the finding either way.

7. **Event ordering for `request_events`** — *New question, not in brief.* Proposal: events returned in the same order they were appended to the live event log (registration / detection order, which today is per-detector iteration order in [internal/scraper/haloce/events](internal/scraper/haloce/events)). Document that ordering is "stream order, not necessarily strict tick order" so a client doing post-hoc analysis knows not to assume `event[i].tick <= event[i+1].tick` for events from the same tick.

8. **Multi-runner writes to `host:all`** — *New question.* Many runners may push summary updates to the aggregate room. Proposal: a single aggregator goroutine owns `host:all` writes; runners post `summaryUpdate` events to a buffered channel, the aggregator coalesces and broadcasts. Keeps the "single goroutine writes per room" invariant intact and avoids lock contention between runners. Aggregator lives in `internal/scraper/manager/` next to the per-instance runners.

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
  Both are M8-class robustness work — not blockers for 5c. File the deferred note here.

## Milestone 6 — PocketBase persistence (with the legacy drop-on-overload bug fixed)

- Port PocketBase collection schemas from legacy `docs/pocketbase.md` into `internal/pocketbase/schema/`: `sessions`, `snapshots`, `events`, `overlay_state`.
- Port `internal/pb/client.go` queue logic **but replace** the silent-drop-on-full behavior with one of:
  - **(a)** Retry with exponential backoff.
  - **(b)** Disk-spool overflow.
  - Decide during port; comment the tradeoff in code.
- Hook scraper events into the new client.
- Verify records land in PocketBase during a full match.

## Milestone 7 — Halo 2 scraper (with known caveats)

- Port `internal/scraper/halo2/*` preserving **every** `UNVERIFIED` comment.
- Known broken areas (each becomes its own follow-up task):
  - Event buffer (`GVAEventCount` always reads 0) — may not exist in xemu's layout; re-derive offsets or find an alternative data source.
  - Objects datum array → real `Alive / Health / Shields / Vehicle` values (currently hardcoded stubs).
  - Team index / primary color / gametype (`SessOffTeamIndex`, `SessOffPrimaryColor`, `GRGVarGameTypeOff`).
- Add runtime offset-sanity checks (base-HVA range check, magic-value probe) so silent bad data becomes a loud error. Apply to **both** scrapers, not just Halo 2.

## Milestone 8 — Robustness + Discord + auth

- Runtime offset validation tightened; loud errors on sanity-check fail.
- Discord bot: slash commands for session start/stop, overlay URLs, who's-playing-now.
- Wrap PocketBase collections with auth (legacy was localhost-only; the template already ships auth middleware).
- Multi-user UX: per-user saved overlay configs + session history.

## Milestone 9+ — Open

- Second-game generalization test (confirm the scraper registry abstraction holds for something non-Halo).
- Community-contributed offset tables (moderation workflow).
- Post-game report UI (replaces HaloCaster's Excel export).
- Hosted / remote deployment story.

---

## Explicit non-goals (for now)

- Desktop GUI (WinForms, DearPyGui) — web is the UI.
- `cmd/{memscan,prove,localproof}` offset-discovery tools — re-derive on demand.
- Halo-specific logic leaking into `internal/xemu/` or the top-level `internal/scraper/` — domain code stays in `internal/scraper/<game>/`.

## Open questions to pin during M2–M6

- **WebSocket format:** adapt legacy `Envelope` to the template's `message.Message`, or extend the template's schema? Decide in M2.
- **PocketBase overload policy:** retry-with-backoff vs. disk-spool. Decide in M6.
- **Podman privilege model:** legacy requires root Podman (KVM + DRI + NET_ADMIN). Keep the requirement or explore rootless (would lose direct device access)? Decide in M3.
- **Deployment model:** same-host (server + xemu on one machine, matches legacy) vs. distributed (thin memory-reader agent + remote PocketBase). Default same-host unless blocked.
