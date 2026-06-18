# Overnight progress log

> Autonomous session started 2026-06-16. Stewart asked for maximum roadmap
> progress overnight: complete M09's reasonable scope, then continue M10, M11,
> onward — each milestone implemented, tested, docs updated, committed locally
> on a stacked branch. **Nothing pushed / merged / PR'd** — all local for review.

This file is the running decision + status log. Read the **Branch stack** and
**Needs Stewart's review** sections first.

## Branch stack (bottom → top, nothing merged to main)

`main` (M08 merged) → `wip/milestone-9` (M09) → `wip/milestone-10` (M10) → `wip/milestone-11` (M11) → `wip/milestone-12` (M12) → `wip/milestone-13` (M13) → `wip/milestone-14` (M14) → `wip/milestone-15` (M15) → `wip/milestone-16` (M16) → `wip/milestone-18` (M18) → …

> M14–M18 (second overnight pass) skip the milestones whose remaining work is
> purely live-hardware-bound (M17 Discord, M19 offset-validation against live
> memory, M20 Halo 2) and land only the unit-testable cores of the rest. The
> branch numbers follow milestone numbers, so M17/M19/M20 are intentionally
> absent from the stack.

Each milestone branches off the previous one's tip. To review in order, walk the
stack bottom-up. Current tip is recorded in the **Status** table below.

## Status

| Milestone | Branch | State | Tests | Notes |
| --------- | ------ | ----- | ----- | ----- |
| M09 — Match-aware kiosk | `wip/milestone-9` | code-complete | green | Live 4-container smoke test can't run here (no podman) |
| M10 — Overlay revamp | `wip/milestone-10` | foundation only | green | 10d filter + schema landed; live overlay UI (10a/b/c/e) deferred — needs live data + OBS |
| M11 — Game minimaps | `wip/milestone-11` | math only | green | projection + height math (`minimap.ts`) landed; assets + renderer + flares deferred — needs assets + live data + OBS |
| M12 — POV marker (stretch) | `wip/milestone-12` | foundation parked | green | perspective-projection kernel (`pov-projection.ts`) landed; rest blocked on 12a camera-offset audit (live xemu) — may defer to M21+ |
| M13 — PB persistence | `wip/milestone-13` | foundation only | green | schema (`series`/`games`/`game_players`) + `internal/games` writer + heuristic landed; `game_events` collision = **decision needed**; Live→Ready wiring + 13d deferred |
| M14 — Series management | `wip/milestone-14` | logic core only | green | `internal/series.Progress` format-termination logic landed; setup/pick-ban/in-progress UIs + live wiring deferred |

## Environment notes (for reproducing my green checks)

- **Use scoped Go commands:** `go build/vet/test ./cmd/... ./internal/...`. A
  bare `./...` walks `containers/browser/config-*` (root-owned leftovers from
  earlier container smoke runs) and dies on `permission denied`. CI uses a clean
  checkout so `./...` is fine there; locally the scoped form is equivalent (all
  Go code lives under `cmd/` + `internal/`).
- Frontend checks run from `sveltekit/`: `pnpm check && pnpm lint && pnpm test && pnpm build`.
- `seed.local.json` is an untracked local file (not mine) — left in place; I use
  explicit `git add <paths>` so it never lands in a commit.
- Stashed on entry: `git stash` "overnight: feat/json-seeder uncommitted seed
  work (02-patch-toml.sh, seed.example.json)" — the working tree was on
  `feat/json-seeder` with 2 uncommitted files when I started; stashed them so I
  could switch to `wip/milestone-9`. Recover with `git stash pop` on
  `feat/json-seeder`.
- Side branches I did **not** touch: `feat/json-seeder`, `chore/align-dev-seed-creds`
  (Stewart's 1-commit dev-seed chore off `main`).

## Decisions made (autonomous — flag anything you'd have called differently)

- M09 stays as committed (`57c566e`), including the WS `host:<name>` room
  narrowing + the kiosk/VNC proxy narrowing. The branch-switch "file modified"
  notes were just the harness observing the checkout, not a revert.

## Needs Stewart's review (does not block overnight work)

- **M09 security boundary:** opening the kiosk HTTP proxy + VNC relay to
  non-admin roster members (`authorizeKioskAccess`). Same predicate also opens
  `host:<name>` WS rooms to roster members. Unit-tested + fails closed, but the
  live stream to a non-admin is unverified (needs the podman smoke test).

## Per-milestone log

### M09 — Match-aware kiosk view (first increment) — `wip/milestone-9`
Committed before this session (`57c566e`). Code-complete; only the live
4-container smoke test remains (podman-gated). No further code changes needed.

### M10 — Overlay revamp + new browser sources — `wip/milestone-10`
Scoped to the **testable data foundation**; live overlay surfaces deferred.
- Landed: 10d dummy-player/neutral-host filter (`internal/scraper/roster`,
  pure + unit-tested), `is_neutral_host` container field, `dummy_gamertags`
  collection (admin-gated, identity.go chain). 10a machine→container lookup
  reuses M9's `MatchContainer` (no new code).
- Deferred (need live multi-instance data + OBS): the overlay UI surfaces
  (10a routing, 10b scoreboards, 10c event popups, 10e POV-correctness pass),
  wiring `FilterRoster` into the live broadcast path, and the 10c animation-
  library choice.
- **Decision:** scoped M10 to the data layer rather than guessing on
  unverifiable overlay UI. The filter is the piece M11/M15 actually depend on.
- **Open question for Stewart (non-blocking):** overlay auth model. Existing
  overlays connect via the authed `scraperWSV2` store; M09 9c narrowed
  `host:<name>` to admin-or-roster-member. Fine for operator/admin-run OBS,
  but if overlays must run from an unauthenticated/token-only OBS instance,
  the room-auth model needs a deliberate design pass.

### M11 — Game minimaps — `wip/milestone-11`
Scoped to the **projection + height math** (the numerically-correct core).
- Landed: `sveltekit/src/lib/utils/minimap.ts` — `projectToMinimap`,
  `aimToScreenAngle`, `heightBand`, `iconScale`, `MAP_TRANSFORMS` registry +
  `transformForMap`. Pure + unit-tested (`minimap.test.ts`).
- Deferred (need traced assets + live data + OBS): the per-map SVG floor
  tracings (11a — registry holds a placeholder Blood Gulch transform with
  uncalibrated values), the `/overlays/minimap/<machine>/` route + renderer
  (11d/e), projectile traces (11f), the 11g canvas-vs-library decision.
- **Decision:** shipped the math, not the renderer. The projection is the part
  that has to be right; the rendering can only be verified by eye over OBS.

### M12 — POV marker overlay (stretch) — `wip/milestone-12`
Stretch-foundation only.
- Landed: `sveltekit/src/lib/utils/pov-projection.ts` — `worldToScreen`
  pinhole perspective projection + frustum cull. Pure + unit-tested.
- Deferred / blocked: 12a camera-offset audit (needs live xemu — if the
  reader lacks a usable camera matrix this becomes an M19 follow-up and M12
  defers to M21+ per the stretch flag), 12b–12e live work.
- **Decision:** parked the projection kernel as a tested foundation rather
  than skip M12, but flagged it explicitly stretch/parked pending the offset
  audit. **For Stewart:** treat M12 as parked, not "in progress" in earnest.

### M13 — PocketBase persistence foundation — `wip/milestone-13`
The biggest **fully-verifiable** milestone of the night (backend + unit tests).
- Landed: `series` / `games` / `game_players` collections (identity.go phase 4,
  FK order); `internal/games.PersistFinishedGame` (writer, auto-creates a
  1-game series) + `SuggestCategory` (13c heuristic). Both unit-tested
  (`PersistFinishedGame` against `tests.NewTestApp()`).
- **DECISION NEEDED (game_events collision):** a `game_events` collection
  already exists — the M5 capture-sink firehose keyed by `instance`. M13's
  spec wants a `game`-FK'd log under the same name. I did NOT touch the
  existing one. Options: (a) extend it with an optional `game` FK + back-fill
  at game-end [my lean]; (b) new `game_event_log` collection; (c) leave events
  in the firehose, join by instance + time range. **This is the one M13 design
  fork that needs Stewart before the persistence path is "done."**
- Deferred (need a live game to verify): wiring `PersistFinishedGame` into the
  scraper Live→Ready transition; 13d durable queue (retry/backoff vs
  disk-spool); `snapshot_blob` format + `game_events` retention choices.

## Where I stopped + recommended next steps

Stopped after M13 — a strong, fully-tested persistence foundation — rather than
push into M14 (series-setup / pick-ban / in-progress UI), which is mostly
frontend + live-state work I can't verify here. Recommended next, in order:
1. **Resolve the M13 `game_events` fork** (above) — unblocks the full
   persistence path + the Live→Ready wiring.
2. **M15 stats aggregation** has a fully-testable backend core (sum
   kills/deaths/wins per gamertag over `game_players`) that builds directly on
   M13 and could land verifiably even before M14's UI.
3. **M14** series-management UI once a live xemu is available to drive it.
4. Run the deferred **live smoke tests** (M09 4-container kiosk, M10/M11 OBS
   overlays, M13 game-end persistence) on a podman-capable host.
