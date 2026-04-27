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

- **2a — full offset audit.** Reconciled all 515 hex constants from `atlas/HaloCaster/HaloCE/halocaster.py` against the 128-offset legacy Go table. Active read-path constants live in [internal/scraper/haloce/offsets.go](internal/scraper/haloce/offsets.go); every other corroborated offset organised by struct in [internal/scraper/haloce/offsets_reference.go](internal/scraper/haloce/offsets_reference.go). Each constant carries a `// halocaster.py:NNN` origin tag. All marked `unverified` until M7's runtime sanity-check pass.
- **2b — scraper code ported.** [reader.go](internal/scraper/haloce/reader.go), [events.go](internal/scraper/haloce/events.go) (19 event types via stat-diff + damage-table fallback), [game.go](internal/scraper/haloce/game.go) (`init()` registers Halo: CE with `scraper.Lookup`), [xboxname.go](internal/scraper/haloce/xboxname.go).
- **2c — WS wiring.** New [internal/scraper/manager](internal/scraper/manager) package owns per-instance lifecycle (Start / Stop / List) and the 30Hz tick goroutine. Decision: **wrap, not extend** — every broadcast becomes `Message{Type:"scraper", Room:"overlay", Payload:<envelope-json>}` so the wire schema stays uniform across all rooms ([loop.go](internal/scraper/manager/loop.go)). New `overlay` room with `RequireAuth` ([rooms/overlay.go](internal/websocket/rooms/overlay.go)). New `Scraper` field on `guards.Services` backed by `internal/guards/interfaces/scraper/` (one-method-per-file).
- **2d — admin routes + main.go wiring.** `GET /api/admin/scraper`, `POST /api/admin/scraper/start`, `POST /api/admin/scraper/stop/{name}` ([routes/scraper](internal/pocketbase/routes/scraper)), all gated by `RequireAuth + RequireAdmin`. `cmd/server/main.go` builds the `Services` skeleton early so the scraper manager gets a stable `*Services` pointer; subsystems mutate fields as they come up. Blank import `_ "internal/scraper/haloce"` triggers the title-ID registration.

### M2 follow-ups (deferred)

- **Snapshot replay for late joiners.** Snapshots only fire on game-state transitions, so a WebSocket client that subscribes mid-game never receives one (and overlay UIs that need map / players / power-item-spawns to render get stuck). Two clean fixes: (a) cache the latest snapshot in `runner` and re-send it when a client `join_room`s the overlay (needs handler integration) or (b) re-emit on a coarse interval (~30s). Pick during M4 when the overlay client is being built.
- **Investigate `power_items: null` in tick payloads.** During the smoke test the initial snapshot's `PowerItemSpawns` came back empty (likely the scenario wasn't fully loaded when the scraper started, since power-item resolution depends on world-object scanning). Worth re-running the smoke test with start-after-match-ready and confirming spawns populate; if they still don't, that's a Halo offset divergence to chase during M7.

### 2a. Offset audit (prerequisite)

The legacy Go offset table has 128 hex constants; HaloCaster's `HaloCE/halocaster.py` has 515 scattered across 2587 lines. Before trusting the legacy table as complete:

1. Read `atlas/HaloCaster/HaloCE/halocaster.py` end-to-end, extracting every memory-offset-like constant with surrounding context (what struct, what field, what read type).
2. Diff the extracted set against `atlas/xemu-cartographer-legacy/internal/scraper/haloce/offsets.go`.
3. Categorize the deltas:
   - Genuinely missing offsets the legacy reader never used → port them.
   - Non-offsets (struct sizes, magic values, indexing math) → document in comments, don't port.
   - Offsets that exist in both but differ in value → investigate (xemu vs. real-Xbox divergence is plausible).
4. Produce a reconciled `internal/scraper/haloce/offsets.go` in the new repo, each offset annotated with its HaloCaster origin (file + line) and verification status.
5. Flag offsets needing runtime verification for Milestone 7's sanity-check work.

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

- **Browser kiosk Firefox crashes inside `jlesage/firefox` container.** xemu container, Selkies stream (port `XemuHTTPS`), and QMP socket discovery all work end-to-end as of the early-M3 port. The `jlesage/firefox` browser container starts but Firefox + xcompmgr fail with `Authorization required, but no authorization protocol specified` / `Cannot open X display!`, leaving the noVNC view at port `BrowserWeb` blank. The kiosk's purpose is to keep an xemu viewer attached at all times (otherwise Selkies idles when no one is watching) — important for the production deployment story but not for memory-bridge testing. Likely fixes to investigate: pin a known-good `jlesage/firefox` tag, pass `USER_ID`/`GROUP_ID` env vars, or replace jlesage with a different always-on viewer (lightweight headless Chromium pointed at the Selkies URL). Revisit during M4 when the frontend overlay is built — may render the kiosk container unnecessary.
- **Discovery → scraper auto-start wiring.** Watcher currently logs `discovery: socket up/down`; the `onAdd` callback needs to spawn a scraper once M1+M2 land.

## Milestone 4 — SvelteKit overlay + container management UI

- New route `sveltekit/src/routes/containers/` — list / create / start / stop / remove containers via the M3 endpoints.
- First real overlay route under `sveltekit/src/routes/overlays/` (likely a players/scoreboard view mirroring legacy `frontend/src/routes/overlays/players/`). Subscribes to the M2 WebSocket stream.
- Skeleton UI components + the template's existing auth store; **do not copy legacy `.svelte` files wholesale**.
- Validate overlay in OBS Browser Source.

## Milestone 5 — PocketBase persistence (with the legacy drop-on-overload bug fixed)

- Port PocketBase collection schemas from legacy `docs/pocketbase.md` into `internal/pocketbase/schema/`: `sessions`, `snapshots`, `events`, `overlay_state`.
- Port `internal/pb/client.go` queue logic **but replace** the silent-drop-on-full behavior with one of:
  - **(a)** Retry with exponential backoff.
  - **(b)** Disk-spool overflow.
  - Decide during port; comment the tradeoff in code.
- Hook scraper events into the new client.
- Verify records land in PocketBase during a full match.

## Milestone 6 — Halo 2 scraper (with known caveats)

- Port `internal/scraper/halo2/*` preserving **every** `UNVERIFIED` comment.
- Known broken areas (each becomes its own follow-up task):
  - Event buffer (`GVAEventCount` always reads 0) — may not exist in xemu's layout; re-derive offsets or find an alternative data source.
  - Objects datum array → real `Alive / Health / Shields / Vehicle` values (currently hardcoded stubs).
  - Team index / primary color / gametype (`SessOffTeamIndex`, `SessOffPrimaryColor`, `GRGVarGameTypeOff`).
- Add runtime offset-sanity checks (base-HVA range check, magic-value probe) so silent bad data becomes a loud error. Apply to **both** scrapers, not just Halo 2.

## Milestone 7 — Robustness + Discord + auth

- Runtime offset validation tightened; loud errors on sanity-check fail.
- Discord bot: slash commands for session start/stop, overlay URLs, who's-playing-now.
- Wrap PocketBase collections with auth (legacy was localhost-only; the template already ships auth middleware).
- Multi-user UX: per-user saved overlay configs + session history.

## Milestone 8+ — Open

- Second-game generalization test (confirm the scraper registry abstraction holds for something non-Halo).
- Community-contributed offset tables (moderation workflow).
- Post-game report UI (replaces HaloCaster's Excel export).
- Hosted / remote deployment story.

---

## Explicit non-goals (for now)

- Desktop GUI (WinForms, DearPyGui) — web is the UI.
- `cmd/{memscan,prove,localproof}` offset-discovery tools — re-derive on demand.
- Halo-specific logic leaking into `internal/xemu/` or the top-level `internal/scraper/` — domain code stays in `internal/scraper/<game>/`.

## Open questions to pin during M2–M5

- **WebSocket format:** adapt legacy `Envelope` to the template's `message.Message`, or extend the template's schema? Decide in M2.
- **PocketBase overload policy:** retry-with-backoff vs. disk-spool. Decide in M5.
- **Podman privilege model:** legacy requires root Podman (KVM + DRI + NET_ADMIN). Keep the requirement or explore rootless (would lose direct device access)? Decide in M3.
- **Deployment model:** same-host (server + xemu on one machine, matches legacy) vs. distributed (thin memory-reader agent + remote PocketBase). Default same-host unless blocked.
