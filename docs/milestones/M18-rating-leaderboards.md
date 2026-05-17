# Milestone 18 — Rating system + multiple leaderboards

> Build a per-game-type rating on top of M15's stats and surface leaderboards on the site + via Discord.

- **18a. Rating algorithm choice.** Decide between TrueSkill, Glicko-2, ELO, or a simpler K-factor system. Trade-offs: TrueSkill handles team/FFA out of the box; Glicko-2 has uncertainty bands; ELO is simplest. Document choice + why in code.
- **18b. Per-game-type rating.** A player has a separate rating per game type (Slayer rating ≠ CTF rating ≠ Oddball rating). Recompute on every game finish via a PB hook on `games` insert.
- **18c. Leaderboard surfaces.** `/leaderboards/<type>/` — game-type, category (only-competitive, only-tournament, all), and time-window (all-time, season, last-30-days) facets. Default landing at `/leaderboards/`.
- **18d. Discord leaderboard commands.** `/leaderboard type:<game> [category:<cat>]` → top-N embed. `/rank user:<gamertag>` → user's current rating + rank. Re-uses M17 plumbing.

Smoke test: play 30 games across 3 game types and 2 categories → ratings update each game-end; leaderboard pages render with correct sort and filtering; Discord commands match the web view.
