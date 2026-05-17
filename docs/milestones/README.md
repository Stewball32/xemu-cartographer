# Milestones

One markdown per milestone. Copy [`_template.md`](_template.md) when starting a new one.

| ID   | Title                                                                                       | Status      |
| ---- | ------------------------------------------------------------------------------------------- | ----------- |
| M00  | [Template cleanup](M00-template-cleanup.md)                                                 | Done        |
| M01  | [xemu memory bridge](M01-xemu-memory-bridge.md)                                             | Done        |
| M02  | [Halo: CE scraper](M02-haloce-scraper.md)                                                   | Done        |
| M03  | [Container lifecycle (Podman)](M03-container-lifecycle.md)                                  | Done        |
| M04  | [SvelteKit overlay + container management UI](M04-frontend-overlay-ui.md)                   | Done        |
| M05  | [Scraper & WebSocket phase model + cache refactor](M05-phase-model-refactor.md)             | Done        |
| M06  | [Frontend polish (theme + auth-refresh fix + debug revamp)](M06-frontend-polish.md)         | In progress |
| M07  | [Identity schemas: gamertags + teams](M07-identity-schemas.md)                              | Planned     |
| M08  | [Roles + permissions](M08-roles-permissions.md)                                             | Planned     |
| M09  | [Match-aware kiosk view](M09-match-aware-kiosk.md)                                          | Planned     |
| M10  | [Overlay revamp + new browser sources](M10-overlay-revamp.md)                               | Planned     |
| M11  | [Game minimaps](M11-game-minimaps.md)                                                       | Planned     |
| M12  | [POV marker overlay (stretch)](M12-pov-marker-overlay.md)                                   | Planned     |
| M13  | [PocketBase persistence foundation: games + series](M13-pb-persistence-games-series.md)     | Planned     |
| M14  | [Series management: setup, pick/ban, in-progress UI](M14-series-management.md)              | Planned     |
| M15  | [Per-user / per-team stats](M15-stats.md)                                                   | Planned     |
| M16  | [Tournament system](M16-tournament-system.md)                                               | Planned     |
| M17  | [Discord integration: stats lookup + per-guild channel posting](M17-discord-integration.md) | Planned     |
| M18  | [Rating system + multiple leaderboards](M18-rating-leaderboards.md)                         | Planned     |
| M19  | [Robustness + offset validation](M19-robustness-offsets.md)                                 | Planned     |
| M20  | [Halo 2 scraper (with known caveats)](M20-halo2-scraper.md)                                 | Planned     |
| M21+ | [Open](M21-plus-open.md)                                                                    | Planned     |

## Status values

`Planned` | `In progress` | `Done` | `Abandoned`

## Conventions

- Filename: `M??-kebab-name.md` (zero-padded to 2 digits).
- Never skip a number. If a milestone is dropped, mark it `Abandoned` and leave the file.
- The `Log` section inside each milestone is **append-only** — never edit past entries.

See [`../README.md`](../README.md) for the full convention.
