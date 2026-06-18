# Milestone 16 — Tournament system

> Tournaments group multiple series with structure (bracket or round-robin). Each tournament round is one series from M14; matches are the games within those series. Bracket rendering on the site.

- **16a. Schema.** `tournaments` `{name, slug, format: "single-elim"|"double-elim"|"round-robin"|"swiss", participants, started_at, ended_at?}`, `tournament_rounds` `{tournament, round_number, series_a, series_b?, winner_advances_to?}`. Series records gain optional FK back. Tournament create gated by `tournament_organizer` role from M8.
- **16b. Bracket / round-robin generators.** Create a tournament → auto-generate the round structure based on participant count + format.
- **16c. Tournament UI.** `/tournaments/[slug]/` rendering bracket or round-robin grid. Live updates as series complete. Click into any round → M14's series-in-progress UI.
- **16d. Tournament-aware series creation.** Inside a tournament, spawning the next round creates a series pre-tagged with `category: tournament` + `tournament + tournament_round` FKs.

Smoke test: 8-team single-elimination → bracket renders, play through round 1 → 4 series + 4+ games persist, bracket auto-advances winners, round 2 series spawn correctly. Repeat with 4-team round-robin. Confirm a non-`tournament_organizer` user gets 403 on `POST /tournaments`.

## Log

_Append-only. Never edit past entries; add a new dated line._

- 2026-06-18: First increment — **16b structure generators**, the pure combinatorial core, implemented + unit-tested during the autonomous overnight run. The schema (16a), bracket UI (16c), tournament-aware series spawning (16d), and the `tournament_organizer` role gate are deferred (need persistence + live updates + the M8 role to verify) — but they all sit on top of this.
  - New `internal/bracket` package:
    - `SingleElim(participants) Bracket` — standard bracket seeding (top two seeds meet only in the final) with first-round byes for the top seeds when the count isn't a power of two. Returns bracket `Size`, total `Rounds` (log2), and the concrete `FirstRound` pairings (later rounds depend on winners). Unit-tested: seed order for 4/8, 3 byes + 1 real match for 5 participants, tiny counts.
    - `RoundRobin(participants) []Round` — circle method; every participant plays every other exactly once, with a per-round bye for odd counts. Unit-tested: 6 unique pairings for 4 players, 3 for 3 (odd).
  - **Deferred:** double-elimination + Swiss generators (double-elim needs winners+losers brackets; Swiss is adaptive round-by-round and can't be fully pre-generated — both are follow-ups); 16a `tournaments` / `tournament_rounds` schema + the `series` back-FK; 16c `/tournaments/[slug]/` bracket/grid UI; 16d tournament-aware series creation (pre-tags `category: tournament` + the round FKs); the `tournament_organizer`-gated `POST /tournaments` route.
