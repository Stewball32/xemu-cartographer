# Status

> **Last updated:** 2026-05-26

The single-pane view of where this project is right now. Update whenever "Now" changes.

## Goals

xemu-cartographer is a real-time game-state scraper for Xbox titles running in [xemu](https://xemu.app/), rebuilt on top of a clean Go + PocketBase + Disgo + SvelteKit template.

Prior implementation is preserved at [atlas/xemu-cartographer-legacy/](../atlas/xemu-cartographer-legacy/). HaloCaster (the older Halo-specific Python/C# sibling) is at [atlas/HaloCaster/](../atlas/HaloCaster/) and holds the richest set of Halo: CE memory offsets. Everything under `atlas/` is **reference-only and must be re-verified before porting** — offsets, patterns, and APIs may have drifted or been wrong to begin with.

Milestones, not dates. Generally each blocks the next, though M03 was ported early (out of sequence) to provide a test substrate for M01+M02 — see [M03 status](milestones/M03-container-lifecycle.md).

## Now

- [ ] [M22 — Moderation + audit log](milestones/M22-moderation-audit.md) (unblocked by M07 7h / [ADR-0002](decisions/0002-unified-audit-log-collection.md))

## Next

Listed by intended execution order (not strict numerical order). M22 and M23 are claimed slots from the M21+ open bucket, drafted alongside M07's scope expansion on 2026-05-26. M07 closed 2026-05-26.

- M23 — Team membership workflows (depends on M07 + M22)
- M08 — Roles + permissions
- M09 — Match-aware kiosk view
- M10 — Overlay revamp + new browser sources
- M11 — Game minimaps
- M12 — POV marker overlay (stretch)
- M13 — PocketBase persistence foundation: games + series
- M14 — Series management: setup, pick/ban, in-progress UI
- M15 — Per-user / per-team stats
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
