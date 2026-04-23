# Roadmap

Migration plan for xemu-cartographer: a real-time game-state scraper for Xbox titles running in [xemu](https://xemu.app/), rebuilt on top of a clean Go + PocketBase + Disgo + SvelteKit template.

Prior implementation is preserved read-only at [.reference/xemu-cartographer-legacy/](.reference/xemu-cartographer-legacy/). HaloCaster (the older Halo-specific Python/C# sibling) is at [.reference/HaloCaster/](.reference/HaloCaster/) and remains the **authoritative source** for Halo: CE memory offsets.

Milestones, not dates. Each blocks the next — nothing ports in parallel.

---

## Milestone 0 — Template cleanup

Bring the fresh template to a clean starting point.

- [x] Rename `stew-site-template` / `github.com/youruser/yourproject` → `xemu-cartographer` / `github.com/Stewball32/xemu-cartographer`.
- [x] Document `.reference/` contents for future Claude sessions.
- [ ] **Follow-up turn** — strip template demo content:
  - Delete `sveltekit/src/routes/examples/`.
  - Drop the `posts` collection + hooks.
  - Remove the placeholder `ping` Discord command.
  - Trim OAuth providers to Discord + GitHub; remove the rest.
  - Reduce seed data to superuser-only.

## Milestone 1 — xemu memory bridge

Foundation. Gets the server able to read memory from any xemu-running Xbox game.

- Port `internal/xemu/{mem.go,qmp.go,instance.go}` from legacy. Port as-is.
- Port `internal/scraper/scraper.go` (game registry + XBE title-ID auto-detection).
- Port `internal/scraper/types.go` (wire `Envelope`) and `state.go` (`TickState`).
- **Smoke test:** manually-started xemu + its QMP socket → title-ID detected → base HVA established → memory reads return plausible values.

## Milestone 2 — Halo: CE scraper

### 2a. Offset audit (prerequisite)

The legacy Go offset table has 128 hex constants; HaloCaster's `HaloCE/halocaster.py` has 515 scattered across 2587 lines. Before trusting the legacy table as complete:

1. Read `.reference/HaloCaster/HaloCE/halocaster.py` end-to-end, extracting every memory-offset-like constant with surrounding context (what struct, what field, what read type).
2. Diff the extracted set against `.reference/xemu-cartographer-legacy/internal/scraper/haloce/offsets.go`.
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

- Copy `containers/xemu/init/{01-setup-toml.sh,02-patch-toml.sh,03-setup-hdd.sh,.env}` verbatim into the new repo's `containers/xemu/init/`.
- Port `internal/podman/{podman.go,ports.go,state.go,ports_test.go}` as-is (clean, no known bugs).
- Port `internal/discovery/` socket-directory watcher; wire it to the scraper registry so new `.sock` files in the shared QMP dir auto-start a scraper.
- Port the 6 `/api/containers/*` HTTP handlers from legacy `cmd/cartographer/main.go` into a new `internal/pocketbase/routes/containers.go`. Adapt to PocketBase's `ServeMux` and add the template's auth middleware (legacy assumed localhost-only).
- Extend `xemu-cartographer.toml.example` or fold container config into the root `.env` / a new `config.toml`; decide during porting.
- **Smoke test:** POST `/api/containers` creates an instance → POST `/start` boots xemu + browser containers → scraper auto-connects → live data flows → POST `/stop` + DELETE tears down cleanly.

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
