# Status

> **Last updated:** 2026-07-01 (M28 broadcast graphics — themed scoreboard + player cards on `feat/broadcast-graphics`, CE mock-verified; live CE pass + H2 theme pending the H2 scraper. Prior: 2026-06-23 Stream Studio hub + game-timer & kill-feed overlays, mock-verified; 2026-06-20 M25 OBS overlay render pages)

The single-pane view of where this project is right now. Update whenever "Now" changes.

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
