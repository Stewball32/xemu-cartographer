# Milestone 15 — Per-user / per-team stats

> Aggregate stats computed from M13's `games` + `game_players` data. Per-user aggregation rolls across the user's gamertags (from M7); per-team aggregation rolls across team trosters.

- **15a. Stats query layer.** Internal helpers (Go-side or PB hooks) for per-gamertag, per-user (all gamertags), per-team aggregations: K/D, W/L, win rate, time played, per-game-type splits.
- **15b. Stats UI.** Profile page at `/u/[username]/` showing stats with filters (game type, category, date range, per-gamertag breakdown). Team page at `/teams/[slug]/` mirroring the same.
- **15c. Match-history view.** Recent games list with links to series + game detail pages. Shareable URLs.
- **15d. Dummy-player override.** UI to mark a `game_players` row as "dummy / neutral host" after the fact, excluding it from aggregates. Reuses the M10d filter at the data layer.

Smoke test: play 5 games across 2 gamertags belonging to the same user → profile page shows correct rolled-up K/D and per-gamertag breakdown; per-game-type filter works; team stats include only games where players were repping that team; a manually-flagged dummy-player row is excluded from aggregates and the match-history view.

## Log

_Append-only. Never edit past entries; add a new dated line._

- 2026-06-18: First increment — **15a stats query + aggregation layer**, fully implemented + unit-tested during the autonomous overnight run (this was the fully-verifiable core flagged at the M13 wrap). The stats UI (15b), match-history view (15c), and dummy-override UI (15d) are deferred — they're SvelteKit pages that need a live data set to verify, but they all consume the helpers below.
  - New `internal/stats` package, split into a pure roll-up + a PB projection:
    - `aggregate.go` (pure, no IO): `Roll(lines) Totals` + `RollByGametype` + `Totals.KD/KDA/WinRate` + `FilterByGametype/Category`. Dummy lines (M10d / M15d `IsDummy`) are excluded from every roll. Per-user = roll a caller's gamertag lines; per-team = roll a roster's lines — both are "feed the right `[]PlayerGame` into `Roll`".
    - `query.go` (PB): `PlayerGamesForGamertag` / `PlayerGamesForGamertags` fetch `game_players` rows and project each into a `PlayerGame` joined to its game (gametype, winner→Won) and the game's series (category) via a nested `game.series` expand. Tested against `tests.NewTestApp()` (winner-team player gets Won=true, loser false, category joins through).
  - **Known limitation (noted in code):** Won compares `winner_team` to the player's team, and an unrecorded winner reads as team 0 — so a draw/no-winner game can credit team 0. A `has_winner`/draw flag on `games` is the follow-up.
  - **Deferred:** 15b profile/team stat pages (`/u/[username]/`, `/teams/[slug]/`) + filters; 15c match-history view; 15d the after-the-fact dummy-flag UI + the `game_players.is_dummy` column (the aggregator already honors the flag; the projection reads it as false until the column lands); per-team resolution (rosters → gamertags → `Roll`) — the `Roll` core is ready, only the roster fan-out query is left.
