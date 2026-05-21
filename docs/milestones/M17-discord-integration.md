# Milestone 17 — Discord integration: stats lookup + per-guild channel posting

> Bot already exists ([internal/disgo/](../internal/disgo/)) but does little besides the placeholder `ping`. This milestone makes it useful: stats commands, automatic posting of game/series/tournament events to configured guild channels.

- **17a. Per-guild config schema.** `discord_guilds` `{guild_id, results_channel?, tournament_channel?, posted_categories: enum-list}`. Admin UI for guild owners to configure (likely a slash command `/cartographer config` rather than a web UI to start).
- **17b. Stats slash commands.** `/stats user:<gamertag>`, `/stats team:<slug>`, `/recent user:<gamertag>` — consume M15 stats helpers, render as Discord embeds via [internal/disgo/components/](../internal/disgo/components/).
- **17c. Event posting.** When a game/series/tournament-round finishes, post an embed to each guild's configured channel (filtered by `posted_categories`). Use the existing `routine.FireAndForget` pattern from PB hooks (CLAUDE.md convention).
- **17d. Tournament announcements.** Bracket-update embed when a tournament round completes; new-tournament announcement on creation.

Subsumes the basic ops commands originally in old M8 (session start/stop, who's-playing-now) — fold those into a single `/cartographer` command group rather than top-level slash commands.

Smoke test: run a tournament series with auto-posting to a test guild — game results post within seconds; `/stats` returns correct data; per-guild category filter prevents casual games from spamming a tournament-only channel.
