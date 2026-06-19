# Overnight progress log

> Autonomous session started 2026-06-16. Stewart asked for maximum roadmap
> progress overnight: complete M09's reasonable scope, then continue M10, M11,
> onward — each milestone implemented, tested, docs updated, committed locally
> on a stacked branch. **Nothing pushed / merged / PR'd** — all local for review.

This file is the running decision + status log. Read the **Branch stack** and
**Needs Stewart's review** sections first.

## Branch stack (bottom → top, nothing merged to main)

`main` (M08 merged) → `wip/milestone-9` (M09) → `wip/milestone-10` (M10) → `wip/milestone-11` (M11) → `wip/milestone-12` (M12) → `wip/milestone-13` (M13) → `wip/milestone-14` (M14) → `wip/milestone-15` (M15) → `wip/milestone-16` (M16) → `wip/milestone-18` (M18) → `wip/game-end-chain` (M13 fork + chain wiring) → `wip/m09-roster-grace` (M09 grace window) → `wip/m10-overlay-auth` (M10 overlay auth) → `wip/m17-discord` (M17 Discord, offline) → …

> `wip/m09-roster-grace` logically belongs to M09 but is stacked on the tip to
> keep the stack linear (no rebase of the whole stack). It can be cherry-picked
> back onto `wip/milestone-9` if you'd rather it land with the rest of M09.
>
> `wip/m10-overlay-auth` (overlay-token auth) stacks on `wip/m09-roster-grace`;
> logically M10, cherry-pickable onto `wip/milestone-10`.
>
> `wip/m17-discord` (M17, built offline) stacks on `wip/m10-overlay-auth`.

### M17 Discord integration (OFFLINE) — `wip/m17-discord` (fifth follow-up)
Stewart added Discord creds (shared test guild w/ NautsLadder) but said **do
not connect the gateway overnight** (rate-limit risk + no human to click). Built
+ tested entirely offline against a test app; live verification deferred.
- **Built + tested:** `discord_guilds` collection + `internal/discordcfg`
  (config + opt-in category filter); `/stats`, `/recent`, `/cartographer`
  (config + status) command definitions; embed builders (`UserStats`,
  `RecentGames`, `GameResult`, `SeriesResult`, `TournamentAnnounce`); a
  `games`-insert PB hook that resolves the series category, filters guild
  configs, and FireAndForget-posts via `svc.Discord.PostEmbed` — **no-op when
  the bot is nil** (so it's inert offline). New `discordiface.Notify.PostEmbed`
  + Bot impl. `/cartographer` gated to Manage-Server users.
- **Decisions:** stats commands are PUBLIC, `/cartographer` is Manage-Server
  gated (config security). `posted_categories` is opt-in (empty = no posts) so
  a guild can't be spammed before it's configured. Slash handlers are thin
  wrappers over unit-tested resolvers (handlers only run on a live interaction).
- **NEEDS LIVE DISCORD (supervised):** connecting the gateway (`NewBot` →
  `syncCommands` is a live REST call — never run offline), registering commands,
  handling interactions, real channel posts. **17d tournament announcements**
  also need the M16 `tournaments` schema (not built yet) — only the embed
  builder exists.
- **Critical reminder for whoever connects it:** xemu + NautsLadder share the
  test guild — watch for double-bot rate limiting when both are live.

### Overlay/spectator auth layer — `wip/m10-overlay-auth` (fourth follow-up)
Stewart settled the OBS overlay-auth design; built the auth layer (render surface
deferred). Resolves the M10 doc's prior "open question".
- **Token** (`internal/overlaytoken`): pure HS256 sign/verify (`token.go`) +
  PB registry mint/active/verify/revoke (`registry.go`) over a new
  `overlay_tokens` collection. Read-only, room-scoped, revocable, audited.
  Default TTL **90 days** (env/per-request tunable); secret from
  `OVERLAY_TOKEN_SECRET` (ephemeral dev fallback).
- **WS handshake = two doors**: user JWT (operator, M09 gate) OR overlay token
  (OBS). Overlay connections are read-only (Hub join/leave whitelist) +
  scoped to their bound instance (`join_room`), barred from the summary feed.
- **Permission both ways**: admins inherently + a new `overlay_manager` M08
  baseline role for non-admin stream helpers (`canManageOverlays`).
- **API** `/api/overlay-tokens` (mint/list/revoke) + minimal `/overlays/manage/`
  UI (mint, Copy URL, revoke).
- Fully unit-tested (token sign/verify incl. expired/revoked/wrong-room,
  read-only whitelist, room scoping, mint permission 403, audit rows). The
  live overlay **render** pages stay deferred (need OBS + a live game).
- **Decision/default to flag:** default token lifetime **90 days** — long-lived
  per the design (revocation is the safety net). Tune via `OVERLAY_TOKEN_TTL_HOURS`.
- **Known cosmetic:** `golang-jwt/jwt/v5` is used directly but still marked
  `// indirect` in go.mod — `go mod tidy` can't run here (it walks the
  root-owned `containers/` artifact dirs and hits permission-denied). Build /
  vet / CI are unaffected. A clean-checkout `go mod tidy` will fix the marker.

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
| M13 — PB persistence | `wip/milestone-13` | foundation only | green | schema + writer + heuristic landed (`game_events` fork resolved + chain wired on `wip/game-end-chain`) |
| M14 — Series management | `wip/milestone-14` | logic core only | green | `internal/series.Progress` format-termination logic landed; setup/pick-ban/in-progress UIs + live wiring deferred |
| M15 — Stats | `wip/milestone-15` | query+agg core | green | `internal/stats` pure roll-up + PB projection landed (unit-tested); stats/match-history/dummy UIs deferred |
| M16 — Tournament | `wip/milestone-16` | generators only | green | `internal/bracket` single-elim + round-robin generators landed (unit-tested); schema/UI/wiring + double-elim/Swiss deferred |
| M18 — Rating + leaderboards | `wip/milestone-18` | algorithm core | green | `internal/rating` Elo + leaderboard ranking landed (unit-tested); recompute hook + leaderboard pages + Discord cmds deferred |
| M13 fork + chain wiring | `wip/game-end-chain` | wired + integ-tested | green | `game_events` option-a + game-end chain (events→series→stats→rating) + Live→Ready trigger; **one live gap** (GameData→FinishedGame mapping) |
| M09 roster grace window | `wip/m09-roster-grace` | done + unit-tested | green | `internal/rostergrace` sliding TTL (default 5 min) on the kiosk/VNC + WS gate; fixes the too-aggressive instant kick on transient roster drop |
| M10 overlay-token auth | `wip/m10-overlay-auth` | done + unit-tested | green | read-only revocable overlay tokens + two-door WS handshake + `overlay_manager` role + mint/revoke API + minimal UI; render surface deferred |
| M17 Discord (offline) | `wip/m17-discord` | offline-complete | green | config + filter + stats/recent/cartographer commands + embeds + games-post hook (no-op offline); gateway NOT connected — live verify deferred |

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
  **Update (2026-06-18):** Stewart live-tested this and it was too aggressive —
  see `wip/m09-roster-grace`. The gate now keeps access for a **5-min sliding
  grace window** after a transient roster drop (only live presence refreshes;
  fail-closed after). Deliberate usability/security trade-off; the `DefaultTTL`
  in `internal/rostergrace` is the knob. Still needs the live smoke test before
  any merge.

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
- **game_events collision — RESOLVED.** Stewart picked option (a). Implemented
  on `wip/game-end-chain` (see the dedicated section below); this flag is closed.
- Deferred (need a live game to verify): the Live→Ready trigger's
  `GameData→FinishedGame` mapping (wired, not live-verified); 13d durable queue;
  `snapshot_blob` format.

### Game-end chain — `wip/game-end-chain` (third follow-up)
Stewart chose option (a) for the `game_events` fork and asked to wire the full
chain. Stacked on `wip/milestone-18`.
- **game_events option-a:** added an optional `game` relation + `idx_game_events_game`
  to the existing firehose collection (shape otherwise unchanged); moved its
  registration into `identity.go` phase 4 (after `games`) since it now relates to
  it; reconciles the column onto an existing collection.
- **Chain (`internal/games.PersistFinishedGame`):** stamp this instance's
  in-window unstamped `game_events` with the game id (idempotent on `game=''`) →
  advance the series (`series.Progress` → stamp `ended_at`) → per-game-type
  two-team Elo (`rating.Update`) into a new `ratings` collection. Returns
  `EventsStamped`/`SeriesStanding`/`RatingsUpdated`.
- **Production trigger:** `runLive` defers `persistFinishedGame(svc)` after
  `captureLiveAsPrevious` (`manager/games_persist.go`), best-effort goroutine.
- **Integration-tested** against `tests.NewTestApp()`: event-window stamping +
  idempotency (2nd persist stamps 0); best-of-3 completes at 2-0 with `ended_at`
  set; winner Elo up / loser down / zero-sum / game counts.
- **Retention decision:** keep `game_events` full; roll-up + prune is the
  documented follow-up.
- **ONE LIVE GAP (flagged, doesn't block):** the `GameData→FinishedGame`
  projection + the trigger firing can only be verified against a real Halo: CE
  match. `internal/games` is fully unit-tested against the same `FinishedGame`
  shape, so the risk is confined to that mapping.

## Where I stopped + recommended next steps

Two passes this session. **Pass 1:** M10–M13. **Pass 2** (after the "keep going"
follow-up): the unit-testable cores of M14, M15, M16, M18. Stopped after M18
because the entire remaining roadmap is genuinely live-hardware-only:
- **M17** (Discord stats/posting) — needs the live bot + a test guild.
- **M19** (offset validation) — needs live xemu memory (the M19 Log is already
  full of live offset-probe work).
- **M20** (Halo 2 scraper) — needs live xemu running Halo 2.

Every milestone M09–M18 now has its tested logic/data core landed; what's left
across all of them is the UI + live-wiring + on-hardware verification. Nothing
more can be landed *green* without xemu/podman/OBS/Discord.

Recommended next, in order:
1. **Resolve the M13 `game_events` fork** — unblocks the M13 Live→Ready wiring,
   which in turn feeds M14d (series attach), M15 (real stats), and M18b (rating
   recompute). This one decision is upstream of a lot.
2. **Wire the tested cores to live data** once a podman/xemu host is available:
   M13 game-end persistence → M14d series attach (`series.Progress`) → M15
   stats (`stats.Roll`) → M18b rating recompute (`rating.Update`). The
   functions are all in place and tested; this is plumbing + a live game.
3. **Build the deferred UIs** on the tested cores: M10/M11 overlays, M14
   series pages, M15 stat/match-history pages, M16 bracket UI, M18 leaderboards.
4. **Run the deferred live smoke tests** (M09 4-container kiosk, M10/M11 OBS
   overlays, M13/M14/M15/M18 game-end → stats/ratings).
5. **Then** the live-only milestones: M17 Discord, M19 offset validation, M20
   Halo 2.
