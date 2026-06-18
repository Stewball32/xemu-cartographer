# Overnight progress log

> Autonomous session started 2026-06-16. Stewart asked for maximum roadmap
> progress overnight: complete M09's reasonable scope, then continue M10, M11,
> onward — each milestone implemented, tested, docs updated, committed locally
> on a stacked branch. **Nothing pushed / merged / PR'd** — all local for review.

This file is the running decision + status log. Read the **Branch stack** and
**Needs Stewart's review** sections first.

## Branch stack (bottom → top, nothing merged to main)

`main` (M08 merged) → `wip/milestone-9` (M09) → `wip/milestone-10` (M10) → `wip/milestone-11` (M11) → `wip/milestone-12` (M12) → …

Each milestone branches off the previous one's tip. To review in order, walk the
stack bottom-up. Current tip is recorded in the **Status** table below.

## Status

| Milestone | Branch | State | Tests | Notes |
| --------- | ------ | ----- | ----- | ----- |
| M09 — Match-aware kiosk | `wip/milestone-9` | code-complete | green | Live 4-container smoke test can't run here (no podman) |
| M10 — Overlay revamp | `wip/milestone-10` | foundation only | green | 10d filter + schema landed; live overlay UI (10a/b/c/e) deferred — needs live data + OBS |
| M11 — Game minimaps | `wip/milestone-11` | math only | green | projection + height math (`minimap.ts`) landed; assets + renderer + flares deferred — needs assets + live data + OBS |

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
