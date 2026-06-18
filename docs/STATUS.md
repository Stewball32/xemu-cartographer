# Status

> **Last updated:** 2026-06-18 (autonomous overnight run — M09 + M10 increments landed on stacked branches; see [overnight-progress.md](overnight-progress.md))

The single-pane view of where this project is right now. Update whenever "Now" changes.

## Goals

xemu-cartographer is a real-time game-state scraper for Xbox titles running in [xemu](https://xemu.app/), rebuilt on top of a clean Go + PocketBase + Disgo + SvelteKit template.

Prior implementation is preserved at [atlas/xemu-cartographer-legacy/](../atlas/xemu-cartographer-legacy/). HaloCaster (the older Halo-specific Python/C# sibling) is at [atlas/HaloCaster/](../atlas/HaloCaster/) and holds the richest set of Halo: CE memory offsets. Everything under `atlas/` is **reference-only and must be re-verified before porting** — offsets, patterns, and APIs may have drifted or been wrong to begin with.

Milestones, not dates. Generally each blocks the next, though M03 was ported early (out of sequence) to provide a test substrate for M01+M02 — see [M03 status](milestones/M03-container-lifecycle.md).

## Now

- [ ] [M09 — Match-aware kiosk view](milestones/M09-match-aware-kiosk.md) — **in progress** (`wip/milestone-9`). 9a/9b/9c implemented end-to-end (membership lookup + `/api/me/match` + `/play/` page + WS & kiosk/VNC proxy roster-narrowing); all CI checks green. Remaining: the live 4-container smoke test (podman-gated, can't run in CI).
- [ ] [M10 — Overlay revamp + new browser sources](milestones/M10-overlay-revamp.md) — **in progress** (`wip/milestone-10`). Data foundation landed: 10d dummy-player filter (`internal/scraper/roster`) + `is_neutral_host` + `dummy_gamertags`. Live overlay surfaces (10a UI / 10b / 10c / 10e) deferred — need live data + OBS.
- [ ] [M11 — Game minimaps](milestones/M11-game-minimaps.md) — **in progress** (`wip/milestone-11`). Projection + height math landed (`minimap.ts`, unit-tested). Traced map assets + renderer + flares deferred — need assets + live data + OBS.
- [ ] [M12 — POV marker overlay (stretch)](milestones/M12-pov-marker-overlay.md) — **foundation parked** (`wip/milestone-12`). Perspective-projection kernel landed (`pov-projection.ts`, unit-tested). Rest blocked on the 12a camera-offset audit (needs live xemu); may defer to M21+ per the stretch flag.
- [ ] [M13 — PocketBase persistence foundation](milestones/M13-pb-persistence-games-series.md) — **in progress** (`wip/milestone-13`). Schema (`series`/`games`/`game_players`) + `internal/games` writer + category heuristic landed (unit-tested). Deferred: the `game_events` collision (decision needed), Live→Ready wiring, 13d queue.
- [ ] [M14 — Series management](milestones/M14-series-management.md) — **in progress** (`wip/milestone-14`). Series-format termination logic (`internal/series.Progress`) landed (unit-tested). Setup / pick-ban / in-progress UIs + live wiring deferred (need live game + UI).
- [ ] [M15 — Per-user / per-team stats](milestones/M15-stats.md) — **in progress** (`wip/milestone-15`). Stats aggregation + query layer (`internal/stats`) landed (unit-tested, pure + PB projection). Stats/match-history/dummy-override UIs deferred (need live data + UI).

## Next

Listed by intended execution order (not strict numerical order). M22 and M23 are claimed slots from the M21+ open bucket, drafted alongside M07's scope expansion on 2026-05-26. M07 closed 2026-05-26; M22 closed 2026-05-31; M23 closed 2026-06-02; M08 closed 2026-06-04.

- M16 — Tournament system
- M17 — Discord integration: stats lookup + per-guild channel posting
- M18 — Rating system + multiple leaderboards
- M19 — Robustness + offset validation
- M20 — Halo 2 scraper (with known caveats)
- M21+ — Open bucket

## Maybe

- **Milestone-open marker tag.** Cut `vX.Y.0-alpha.0` as the first commit of every new milestone so `git describe` (and therefore `/api/version`) self-identifies which milestone the running binary belongs to: `v0.7.0-alpha.0-5-gXXXX` reads as "5 commits into M7." Escalate to `-alpha.1` / `-beta.1` / `-rc.1` only for shareable mid-milestone checkpoints; cut `vX.Y.0` at milestone close. Promote to an ADR (likely 0002) once the workflow's been used on a milestone or two and pulls its weight.

## Out of scope

- Desktop GUI (WinForms, DearPyGui) — web is the UI.
- `cmd/{memscan,prove,localproof}` offset-discovery tools — re-derive on demand.
- Halo-specific logic leaking into `internal/xemu/` or the top-level `internal/scraper/` — domain code stays in `internal/scraper/<game>/`.
