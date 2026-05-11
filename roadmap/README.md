# Roadmap

Migration plan for xemu-cartographer: a real-time game-state scraper for Xbox titles running in [xemu](https://xemu.app/), rebuilt on top of a clean Go + PocketBase + Disgo + SvelteKit template.

Prior implementation is preserved at [atlas/xemu-cartographer-legacy/](../atlas/xemu-cartographer-legacy/). HaloCaster (the older Halo-specific Python/C# sibling) is at [atlas/HaloCaster/](../atlas/HaloCaster/) and holds the richest set of Halo: CE memory offsets. Everything under `atlas/` is **reference-only and must be re-verified before porting** — offsets, patterns, and APIs may have drifted or been wrong to begin with.

Milestones, not dates. Generally each blocks the next, though M3 was ported early (out of sequence) to provide a test substrate for M1+M2 — see [M3 status](M3-container-lifecycle.md).

## Index

| #     | Milestone                                                                                  | Status       |
| ----- | ------------------------------------------------------------------------------------------ | ------------ |
| M0    | [Template cleanup](M0-template-cleanup.md)                                                 | Done         |
| M1    | [xemu memory bridge](M1-xemu-memory-bridge.md)                                             | Ported       |
| M2    | [Halo: CE scraper](M2-haloce-scraper.md)                                                   | Ported       |
| M3    | [Container lifecycle (Podman)](M3-container-lifecycle.md)                                  | Ported early |
| M4    | [SvelteKit overlay + container management UI](M4-frontend-overlay-ui.md)                   | Ported       |
| M5    | [Scraper & WebSocket phase model + cache refactor](M5-phase-model-refactor.md)             | Implemented  |
| M6    | [Frontend polish (theme + auth-refresh fix + debug revamp)](M6-frontend-polish.md)         | Planned      |
| M7    | [Identity schemas: gamertags + teams](M7-identity-schemas.md)                              | Planned      |
| M8    | [Roles + permissions](M8-roles-permissions.md)                                             | Planned      |
| M9    | [Match-aware kiosk view](M9-match-aware-kiosk.md)                                          | Planned      |
| M10   | [Overlay revamp + new browser sources](M10-overlay-revamp.md)                              | Planned      |
| M11   | [Game minimaps](M11-game-minimaps.md)                                                      | Planned      |
| M12   | [POV marker overlay (stretch)](M12-pov-marker-overlay.md)                                  | Planned      |
| M13   | [PocketBase persistence foundation: games + series](M13-pb-persistence-games-series.md)    | Planned      |
| M14   | [Series management: setup, pick/ban, in-progress UI](M14-series-management.md)             | Planned      |
| M15   | [Per-user / per-team stats](M15-stats.md)                                                  | Planned      |
| M16   | [Tournament system](M16-tournament-system.md)                                              | Planned      |
| M17   | [Discord integration: stats lookup + per-guild channel posting](M17-discord-integration.md) | Planned     |
| M18   | [Rating system + multiple leaderboards](M18-rating-leaderboards.md)                        | Planned      |
| M19   | [Robustness + offset validation](M19-robustness-offsets.md)                                | Planned      |
| M20   | [Halo 2 scraper (with known caveats)](M20-halo2-scraper.md)                                | Planned      |
| M21+  | [Open](M21-plus-open.md)                                                                   | Open bucket  |

## Explicit non-goals (for now)

- Desktop GUI (WinForms, DearPyGui) — web is the UI.
- `cmd/{memscan,prove,localproof}` offset-discovery tools — re-derive on demand.
- Halo-specific logic leaking into `internal/xemu/` or the top-level `internal/scraper/` — domain code stays in `internal/scraper/<game>/`.

## Open questions to pin during M2–M13

- **WebSocket format:** adapt legacy `Envelope` to the template's `message.Message`, or extend the template's schema? Decide in M2.
- **PocketBase overload policy:** retry-with-backoff vs. disk-spool. Decide in M13.
- **Podman privilege model:** legacy requires root Podman (KVM + DRI + NET_ADMIN). Keep the requirement or explore rootless (would lose direct device access)? Decide in M3.
- **Deployment model:** same-host (server + xemu on one machine, matches legacy) vs. distributed (thin memory-reader agent + remote PocketBase). Default same-host unless blocked.
