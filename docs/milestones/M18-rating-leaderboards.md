# Milestone 18 — Rating system + multiple leaderboards

> Build a per-game-type rating on top of M15's stats and surface leaderboards on the site + via Discord.

- **18a. Rating algorithm choice.** Decide between TrueSkill, Glicko-2, ELO, or a simpler K-factor system. Trade-offs: TrueSkill handles team/FFA out of the box; Glicko-2 has uncertainty bands; ELO is simplest. Document choice + why in code.
- **18b. Per-game-type rating.** A player has a separate rating per game type (Slayer rating ≠ CTF rating ≠ Oddball rating). Recompute on every game finish via a PB hook on `games` insert.
- **18c. Leaderboard surfaces.** `/leaderboards/<type>/` — game-type, category (only-competitive, only-tournament, all), and time-window (all-time, season, last-30-days) facets. Default landing at `/leaderboards/`.
- **18d. Discord leaderboard commands.** `/leaderboard type:<game> [category:<cat>]` → top-N embed. `/rank user:<gamertag>` → user's current rating + rank. Re-uses M17 plumbing.

Smoke test: play 30 games across 3 game types and 2 categories → ratings update each game-end; leaderboard pages render with correct sort and filtering; Discord commands match the web view.

## Log

_Append-only. Never edit past entries; add a new dated line._

- 2026-06-18: First increment — the **rating algorithm + leaderboard ranking** (18a + the 18c sort core), implemented + unit-tested during the autonomous overnight run. The per-game-type recompute hook (18b), leaderboard pages (18c UI), and Discord commands (18d) are deferred (need persistence + the live bot to verify) but all consume these pure functions.
  - **18a decision — Elo** (documented in `elo.go`), chosen over Glicko-2 / TrueSkill for v1: simplest to test deterministically, zero-sum (no rating inflation), adequate for 1v1 / two-team Halo. Trade-offs noted in code; FFA (N-way) rating is a follow-up.
  - New `internal/rating` package: `Expected(a, b)` (logistic win prob) + `Update(a, b, scoreA, k)` (zero-sum Elo update — `newA+newB == a+b`; upsets move more than expected results). `DefaultRating = 1500`, `DefaultK = 32`. Per-game-type (18b) is the caller's concern — it keys (rating, gametype) and feeds the matching pair in; the math is game-type agnostic. Leaderboard ranking: `Rank` (rating desc, tiebreak by games then gamertag, input-immutable), `TopN`, `RankOf`. All unit-tested.
  - **Deferred:** 18b's PB hook on `games` insert that recomputes both players' per-game-type ratings; 18c `/leaderboards/<type>/` pages with category + time-window facets; 18d `/leaderboard` + `/rank` Discord commands (ride on M17's plumbing, which is itself deferred); FFA rating + per-game-type K tuning.
