# Status

> **Last updated:** 2026-08-30 (everything below SHIPPED to `main` + prod). Prior: 2026-07-02 live CE+H2 offset verification on the 3way-systemlink rig — see **Live offset verification** below.

The single-pane view of where this project is right now. Update whenever "Now" changes.

## Shipped 2026-08-29/30 — the four-branch release (PR #33, merge `a5a7de9`)

The OBS overlay re-skin, the organizer six-page redesign (M29), the settings
five-tab consolidation (M30), and the Halo 2 offsets unblock (M20) all landed on
`main` via PR #33 and are **live in prod** (`/srv/http/xemu-cartographer/prod`,
commit `cce741a`, cutover 2026-08-29 20:58 PDT — 7 migrations applied). The
`update/*` feature branches they were authored on are merged and deleted; only
`main` and `beta` remain. Per-milestone detail in the sections below.

**Still open across that release:** Gametypes/Rulesets field truing against
in-game screenshots + a live pass with a real ingested disc (M29); the Teams tab
design that un-parks the membership surface, and the H2 profile control-byte
mapping that enables the settings Controls panel (M30); H2 assists/score/
kill-streak/team-scores and the Slim client/server offset sets (M20).

## M30 player settings redesign

The `/gamertag/` page and the old `/settings/` layout consolidated into one
five-tab Settings page (General · Halo: CE (WIP) · Halo 2 (WIP) · Stream ·
Accounts). New stream identity: `users.motto` + `users.nameplate` (curated
picker over the M29 nameplates pool, selectable-guard hook), served to overlays
through `/api/public/profiles` → `player.motto`/`player.plateBg` on the plate.
Default-gamertag changes now sync `users.gamertag` and regenerate both signed
profiles. **Parked by decision**: the old Teams/membership settings surface
(components stay in-tree, awaiting the Teams tab design). See
[milestones/M30-settings-redesign.md](milestones/M30-settings-redesign.md).

## M29 organizer route redesign

The `/organizer/` tabs became six rail pages (designer handoff): **Offsets**
(runtime offset-set imports, delete-with-migration, scraper dynamic source),
**Discs** (role model play/server/shelved + allow-on-xbox, replaces the
available bool), **Maps** (canonical build catalog keyed game+filename+hash,
variant curation, power items), **Gametypes** (creator absorbed; library vs
in-game name; server-schema fields), **Rulesets** (gametypes + pool + size +
series), **Nameplates** (600×100 banner library with the exact-geometry plate
crop; text-outline deviation back-ported to the overlay NamePlate).
Smoke-verified on a fresh dev DB; remaining: Gametypes/Rulesets field truing
against in-game screenshots + a live pass with a real ingested disc. See
[milestones/M29-organizer-redesign.md](milestones/M29-organizer-redesign.md).

## Live offset verification (2026-07-02) — CE + H2, unblocks M19/M20

Ran a live xemu offset-verification session on the `scripts/3way-systemlink/` rig
(branch `feat/halo2-scraper`) + the `../halo-offset-mapper` runtime tooling. Findings +
provenance committed & pushed in **halo-offset-mapper** (`docs/ce-offset-mapping-2026-07-02.md`,
`docs/h2-offset-confirm-2026-07-02.md`).

- **Rig proven from cold:** `udp_reflector.py` + `launch-instance.sh {1,2}` → 2 CE
  instances → **system link confirmed end-to-end** (host game discovered across the
  reflector, join, START) → live 2-player Blood Gulch Slayer FFA match. `launch-h2.sh`
  → H2 boots + drives to a live splitscreen match.
- **CE:** foundation/player/biped/weapon/object + system-link network layer offsets all
  re-verified live (zero corrections). NEW: `OffBipedCrouchScale 0x464` static-derived →
  **runtime-verified** (crouch induced); `AddrScoreSlayer 0x276710` **FFA per-player
  indexing confirmed** (prior open item); attributed cross-player kill reconfirmed on
  Blood Gulch (client stayed connected — no desync on open FFA).
- **H2:** phase-1 GameReader offsets **runtime-verified live** in an H2 match (Turf) —
  players/objects arrays, biped health/shield/max(45/70)/pos/frags(2), Battle Rifle
  mag 36/reserve 72, title id `0x4D530064`. This is the H2 scraper read-path piece that
  was "unverified without xemu".
- **Reusable tooling added** (halo-offset-mapper): `scripts/runtime/ce_lobby.sh`
  (system-link lobby driver — gated building blocks; menu nav must be screenshot-verified,
  not reliably hands-free) + a `tag-handle` schema-kind fix (unblocked `offsetmap validate`).
- **Remaining:** multi-instance **H2 system link** (reflector recipe applies; needs a
  2-console H2 match) + H2 combat-stat induction; CE objective-tick values (CTF capture /
  Oddball possession — physics enter-edge wall); vehicle-seat / overshield / FFA-leader /
  friendly-fire / time-limit. Robust hands-free menu nav would need screenshot/OCR gating.

## Repo housekeeping (2026-07-01)

Patchwork/backup pass over the 6-worktree working set. **Nothing was merged into `main`**; all changes are committed on their own branches and pushed.

- **Backup (was the top risk):** `main` (74 commits ahead of origin, unpushed) + all 17 local feature/chore branches are now pushed to `origin` and tracking. Previously only an old `main` existed on the remote.
- **In-progress work committed:**
  - `chore/xemu-test-harness` — harness now uses xemu 0.8.136's native `xemu` display backend instead of stock-QEMU `sdl` (which that build doesn't compile), fixing the visible-window launch.
  - `feat/spartan-skinfix-ce-markv` + `feat/spartan-real-poses` — added `tools/h2-model/.gitignore` (mirrors `tools/ce-model/`) so `__pycache__/` + `out/mcc/composite_cmp/` render artifacts stop showing as untracked.
- **Build/test:** green on `feat/broadcast-graphics` (= `main` + M28) — Go `build`/`vet`/all tests; frontend `lint`/`check` (0 err)/`test` (282)/`build`.
- **Stashes triaged — all preserved, none applied:**
  - `stash@{0}` (orphaned, ex-`feat/stream-assets-studio`) — **obsolete**: the visualizer `?map=` demo-mode WIP was finished + merged as `223c718` and has since evolved (`mockStagedModel`). Safe to drop; left in place.
  - `stash@{1}` (`feat/json-seeder`) — its `02-patch-toml.sh` netif fix is **already on `main`** (functionally identical); the only real delta is a `seed.example.json` credential change → **Stewart's call**.
  - `stash@{2}` (`chore/align-dev-seed-creds`) — `data.go` seed-credential approach → **Stewart's call** (competes with json-seeder; deliberately left, per the deferred-seeder decision).
- **TODO/FIXME:** only 3 in source (Halo:CE offset provenance + an M7 live-verify note) — none quick/safe (all need live xemu). Left as-is.

**⚠️ Directory move:** this repo drives 6 git worktrees under `/home/stew/repos/` (`xemu-cartographer`, `xemu-cart-markv`, `xemu-cartographer-emblem-fix`, `-harness`, `-overlay`, `-spartan-poses`). After moving the parent dir, run `git worktree repair` from the main checkout (passing the moved worktree paths) to fix the absolute gitdir links — otherwise the linked worktrees detach.

## Goals

xemu-cartographer is a real-time game-state scraper for Xbox titles running in [xemu](https://xemu.app/), rebuilt on top of a clean Go + PocketBase + Disgo + SvelteKit template.

Prior implementation is preserved at [atlas/xemu-cartographer-legacy/](../atlas/xemu-cartographer-legacy/). HaloCaster (the older Halo-specific Python/C# sibling) is at [atlas/HaloCaster/](../atlas/HaloCaster/) and holds the richest set of Halo: CE memory offsets. Everything under `atlas/` is **reference-only and must be re-verified before porting** — offsets, patterns, and APIs may have drifted or been wrong to begin with.

Milestones, not dates. Generally each blocks the next, though M03 was ported early (out of sequence) to provide a test substrate for M01+M02 — see [M03 status](milestones/M03-container-lifecycle.md).

## Now

- [ ] [M09 — Match-aware kiosk view](milestones/M09-match-aware-kiosk.md) — **in progress** (`wip/milestone-9`). 9a/9b/9c implemented end-to-end (membership lookup + `/api/me/match` + `/play/` page + WS & kiosk/VNC proxy roster-narrowing); all CI checks green. Remaining: the live 4-container smoke test (podman-gated, can't run in CI).
- [ ] [M10 — Overlay revamp + new browser sources](milestones/M10-overlay-revamp.md) — **in progress** (`wip/milestone-10`; overlay-token auth on `wip/m10-overlay-auth`). Data foundation (10d filter + schema) + the **overlay/spectator auth layer** (read-only revocable tokens + two-door WS handshake + `overlay_manager` role + mint/revoke API + minimal UI) landed. The first **render surfaces** (scoreboard + status strip) landed under M25; remaining render surfaces (10a POV routing / 10c events / 10e POV pass) deferred — need live data + OBS.
- [ ] [M25 — OBS spectator overlays](milestones/M25-obs-overlays.md) — **in progress** (`wip/obs-spectator-overlays`). Scoreboard/roster (`/overlays/[instance]/`) + match-status strip (`/overlays/[instance]/status/`) render pages on the shipped M10 overlay-token contract, with `?mock=1` sample mode; mint page emits both URLs. Pure builders unit-tested; check/lint/test/build + headless mock render green. Remaining: live match pass (K/D/A + team-score correctness) + wiring the M10d dummy filter into the broadcast.
- [ ] **Stream Studio — stream-asset hub** (`feat/stream-assets-studio`, off `feat/visualizer-3d`). New `/studio/` gallery maps every browser-source asset (scoreboard, status strip, players, 2D/3D visualizers) with a live `?mock=1` inline preview + one-click scoped-token "Copy OBS URL" per asset, driven by a registry (`stream-assets.ts`) where adding an asset = a route + an entry. Seeds two new transparent overlays — **game timer** (`/overlays/[instance]/timer/`) + **kill feed** (`/overlays/[instance]/killfeed/`). Pure builders unit-tested; check/lint/test/build + headless mock renders green. Carries the overlay-token `host:*` room auth fix so copy-link minting works. Docs: [STREAM-ASSETS.md](STREAM-ASSETS.md). Remaining: live-match pass (kill-feed events + token URLs against a real game).
- [ ] [M28 — Broadcast graphics](milestones/M28-broadcast-graphics.md) — **in progress** (`feat/broadcast-graphics`). Two themed "hero" OBS sources on the M25/Studio plumbing: **broadcast scoreboard** (`/overlays/[instance]/scoreboard/`) + **player cards** (`/overlays/[instance]/cards/`, plus `card/[slot]/` spotlight), using the real Spartan/Elite busts + H2 emblems + exact armor palettes. A `?game=ce|h2` switch (`--bc-*` theme layer) renders CE amber/green vs H2 blue from the same data. New pure helpers unit-tested (theme + params, +12); check/lint/test (273)/build + headless mock renders in both themes green. **CE live-capable now; H2 theme preview-only** until the H2 scraper (M20) provides live roster/scores/emblems.
- [ ] [M11 — Game minimaps](milestones/M11-game-minimaps.md) — **in progress** (`wip/milestone-11`). Projection + height math landed (`minimap.ts`, unit-tested). Traced map assets + renderer + flares deferred — need assets + live data + OBS.
- [ ] [M12 — POV marker overlay (stretch)](milestones/M12-pov-marker-overlay.md) — **foundation parked** (`wip/milestone-12`). Perspective-projection kernel landed (`pov-projection.ts`, unit-tested). Rest blocked on the 12a camera-offset audit (needs live xemu); may defer to M21+ per the stretch flag.
- [ ] [M13 — PocketBase persistence foundation](milestones/M13-pb-persistence-games-series.md) — **in progress** (`wip/milestone-13`; chain wired on `wip/game-end-chain`). Schema + writer + heuristic landed. **`game_events` fork resolved (option a)** + full game-end chain wired (events stamping + series advance + rating) with the Live→Ready trigger; integration-tested. Remaining: live verification, 13d durable queue.
- [ ] [M14 — Series management](milestones/M14-series-management.md) — **in progress** (`wip/milestone-14`). `internal/series.Progress` landed; **14d completion wiring landed** in the game-end chain. Setup / pick-ban / in-progress UIs deferred.
- [ ] [M15 — Per-user / per-team stats](milestones/M15-stats.md) — **in progress** (`wip/milestone-15`). Stats aggregation + query layer (`internal/stats`) landed (unit-tested); now reads real persisted games via the chain. Stats/match-history/dummy-override UIs deferred.
- [ ] [M16 — Tournament system](milestones/M16-tournament-system.md) — **in progress** (`wip/milestone-16`). Bracket generators (`internal/bracket`: single-elim + round-robin) landed (unit-tested). Schema/UI/wiring + double-elim/Swiss deferred.
- [ ] [M18 — Rating system + leaderboards](milestones/M18-rating-leaderboards.md) — **in progress** (`wip/milestone-18`; recompute wired on `wip/game-end-chain`). Elo + leaderboard ranking landed; **18b recompute wired** + `ratings` store added. Leaderboard pages, Discord cmds deferred.
- [ ] [M17 — Discord integration](milestones/M17-discord-integration.md) — **in progress, offline** (`wip/m17-discord`). `discord_guilds` config + `internal/discordcfg` filter + stats/recent/cartographer commands + embed builders + the `games`-insert post hook (FireAndForget, no-op offline) all built + unit/integration-tested. **Gateway intentionally not connected** — live verification (command sync, interactions, real posts, 17d tournament wiring) needs a supervised session.

## Next

Listed by intended execution order (not strict numerical order). M22 and M23 are claimed slots from the M21+ open bucket, drafted alongside M07's scope expansion on 2026-05-26. M07 closed 2026-05-26; M22 closed 2026-05-31; M23 closed 2026-06-02; M08 closed 2026-06-04; M26 (HDD copy-on-write overlays) closed 2026-06-20.

- M19 — Robustness + offset validation (needs live xemu memory)
- M20 — Halo 2 scraper (with known caveats)
- M21+ — Open bucket

## Maybe

- **Milestone-open marker tag.** Cut `vX.Y.0-alpha.0` as the first commit of every new milestone so `git describe` (and therefore `/api/version`) self-identifies which milestone the running binary belongs to: `v0.7.0-alpha.0-5-gXXXX` reads as "5 commits into M7." Escalate to `-alpha.1` / `-beta.1` / `-rc.1` only for shareable mid-milestone checkpoints; cut `vX.Y.0` at milestone close. Promote to an ADR (likely 0002) once the workflow's been used on a milestone or two and pulls its weight.

## Out of scope

- Desktop GUI (WinForms, DearPyGui) — web is the UI.
- `cmd/{memscan,prove,localproof}` offset-discovery tools — re-derive on demand.
- Halo-specific logic leaking into `internal/xemu/` or the top-level `internal/scraper/` — domain code stays in `internal/scraper/<game>/`.
