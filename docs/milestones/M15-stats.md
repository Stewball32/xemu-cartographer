# Milestone 15 — Per-user / per-team stats

> Aggregate stats computed from M13's `games` + `game_players` data. Per-user aggregation rolls across the user's gamertags (from M7); per-team aggregation rolls across team trosters.

- **15a. Stats query layer.** Internal helpers (Go-side or PB hooks) for per-gamertag, per-user (all gamertags), per-team aggregations: K/D, W/L, win rate, time played, per-game-type splits.
- **15b. Stats UI.** Profile page at `/u/[username]/` showing stats with filters (game type, category, date range, per-gamertag breakdown). Team page at `/teams/[slug]/` mirroring the same.
- **15c. Match-history view.** Recent games list with links to series + game detail pages. Shareable URLs.
- **15d. Dummy-player override.** UI to mark a `game_players` row as "dummy / neutral host" after the fact, excluding it from aggregates. Reuses the M10d filter at the data layer.

Smoke test: play 5 games across 2 gamertags belonging to the same user → profile page shows correct rolled-up K/D and per-gamertag breakdown; per-game-type filter works; team stats include only games where players were repping that team; a manually-flagged dummy-player row is excluded from aggregates and the match-history view.
