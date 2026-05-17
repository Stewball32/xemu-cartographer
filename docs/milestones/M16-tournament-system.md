# Milestone 16 — Tournament system

> Tournaments group multiple series with structure (bracket or round-robin). Each tournament round is one series from M14; matches are the games within those series. Bracket rendering on the site.

- **16a. Schema.** `tournaments` `{name, slug, format: "single-elim"|"double-elim"|"round-robin"|"swiss", participants, started_at, ended_at?}`, `tournament_rounds` `{tournament, round_number, series_a, series_b?, winner_advances_to?}`. Series records gain optional FK back. Tournament create gated by `tournament_organizer` role from M8.
- **16b. Bracket / round-robin generators.** Create a tournament → auto-generate the round structure based on participant count + format.
- **16c. Tournament UI.** `/tournaments/[slug]/` rendering bracket or round-robin grid. Live updates as series complete. Click into any round → M14's series-in-progress UI.
- **16d. Tournament-aware series creation.** Inside a tournament, spawning the next round creates a series pre-tagged with `category: tournament` + `tournament + tournament_round` FKs.

Smoke test: 8-team single-elimination → bracket renders, play through round 1 → 4 series + 4+ games persist, bracket auto-advances winners, round 2 series spawn correctly. Repeat with 4-team round-robin. Confirm a non-`tournament_organizer` user gets 403 on `POST /tournaments`.
