# Milestone 17 — Discord integration: stats lookup + per-guild channel posting

> Bot already exists ([internal/disgo/](../internal/disgo/)) but does little besides the placeholder `ping`. This milestone makes it useful: stats commands, automatic posting of game/series/tournament events to configured guild channels.

- **17a. Per-guild config schema.** `discord_guilds` `{guild_id, results_channel?, tournament_channel?, posted_categories: enum-list}`. Admin UI for guild owners to configure (likely a slash command `/cartographer config` rather than a web UI to start).
- **17b. Stats slash commands.** `/stats user:<gamertag>`, `/stats team:<slug>`, `/recent user:<gamertag>` — consume M15 stats helpers, render as Discord embeds via [internal/disgo/components/](../internal/disgo/components/).
- **17c. Event posting.** When a game/series/tournament-round finishes, post an embed to each guild's configured channel (filtered by `posted_categories`). Use the existing `routine.FireAndForget` pattern from PB hooks (CLAUDE.md convention).
- **17d. Tournament announcements.** Bracket-update embed when a tournament round completes; new-tournament announcement on creation.

Subsumes the basic ops commands originally in old M8 (session start/stop, who's-playing-now) — fold those into a single `/cartographer` command group rather than top-level slash commands.

Smoke test: run a tournament series with auto-posting to a test guild — game results post within seconds; `/stats` returns correct data; per-guild category filter prevents casual games from spamming a tournament-only channel.

## Log

_Append-only. Never edit past entries; add a new dated line._

- 2026-06-19: **M17 built OFFLINE** (`wip/m17-discord`, stacked on the M10-overlay-auth work). Per Stewart's instruction the live gateway was **not** connected (xemu + NautsLadder share the test guild → rate-limit risk, and no human to click interactions) — everything is implemented + unit/integration-tested against a test app; live verification (gateway connect, command sync, interaction handling, real posts) is deferred to a supervised session.
  - **17a — per-guild config.** New `discord_guilds` collection (`guild_id` unique, `results_channel`, `tournament_channel`, `posted_categories` opt-in multi-select; nil rules, server-side access). New `internal/discordcfg` package: `Get`/`Upsert`/`All` + the pure category filter (`GuildConfig.ResultsTarget(category)` + `ResultsTargets(configs, category)` fan-out). `posted_categories` is opt-in (empty = post nothing) so no guild is spammed until configured. Unit-tested (filter + upsert/get/all).
  - **17b — stats commands.** `/stats user:<gamertag> | team:<slug>` + `/recent user:<gamertag>` command definitions + handlers. Embed builders in `components/embeds/` (`UserStats`, `RecentGames` — pure, tested) consume the M15 `internal/stats` layer; the resolvers (`userStatsEmbed`, `recentEmbed`, `teamStatsEmbed`) are tested against a test app. Handlers are thin option-plumbing (live-only).
  - **17c — event posting.** `embeds.GameResult` builder + a new PB hook (`games` after-create, `games_discord_post.go`) that resolves the series category, filters guild configs, and FireAndForget-posts to each opted-in channel via `svc.Discord.PostEmbed`. **No-op when the bot isn't connected** (offline / tests), so it never affects persistence. The resolution + category filter (`gameResultTargets`) is unit-tested; the actual REST post is the live gap. New `discordiface.Notify.PostEmbed` + Bot impl + `actions.PostEmbed`.
  - **/cartographer group** (folds the old-M8 ops commands): `config` (writes the per-guild config — `applyConfig` + `parseCategories` tested) and `status` (who's-playing-now from the scraper roster — `statusEmbed` tested). Gated to **Manage-Server** users via `DefaultMemberPermissions` (so only guild admins reconfigure posting); `/stats` + `/recent` stay public.
  - **17d — tournament announcements.** `embeds.TournamentAnnounce` builder is ready, but the **posting + wiring are deferred**: M16's `tournaments`/`tournament_rounds` collections don't exist yet (only the bracket generators), so there's nothing to hook. Needs the M16 schema + a live gateway.
  - **Deferred / needs a supervised live session:** connecting the gateway (`NewBot` → `syncCommands` is a live REST call — never run offline), command registration with Discord, interaction handling (the thin handlers only run live), real channel posts, and the 17d tournament wiring. The smoke test (auto-post a tournament series to the test guild; `/stats` returns correct data; category filter prevents spam) is the live verification.
