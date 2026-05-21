# Milestone 13 — PocketBase persistence foundation: games + series

> Replaces the original M6 "port snapshots/events/sessions/overlay_state" framing. The new data model is **game** (singular contest) + **series** (a group of one or more games with a format and a category). Pickup matches are 1-game series; tournament rounds are best-of-N or first-to-X series. Categorization is a field on series, defaulted by a gametype-variant-name heuristic and editable after the fact.
>
> **Terminology:** "game" = singular contest (one round of Slayer, one CTF round). "Series" = a grouping of one or more games with a format and category.

## 13a. Schema design

Collections under [internal/pocketbase/schema/](../internal/pocketbase/schema/), one file each:

- `series` — `{name?, format: "exact-N"|"first-to-X", target_n, category: enum, created_by, started_at, ended_at?, tournament?, tournament_round?}`. Category enum at minimum: `casual | competitive | tournament | custom`.
- `games` — `{series, container, host_machine_name, map, gametype, variant_name, started_at, ended_at, winner_team?, score_summary, snapshot_blob?}`.
- `game_events` — `{game, tick, type, payload}`. Append-only event log.
- `game_players` — `{game, gamertag, team, kills, deaths, assists, score, time_alive_ms, weapon_loadout?}`. One row per player per game.

Decisions to lock during 13a: snapshot blob format (full instanceCache JSON vs trimmed?), retention policy on `game_events` (full forever vs roll up to `game_players` + drop after N days?), how series surface "in progress" state (an absence of `ended_at` plus a join-table to active games?).

## 13b. Game-end persistence wiring

Hook into M5 manager's Live → Ready transition (the path that already populates `cache.PreviousGame`). Write a `games` row + N `game_players` + the event stream. If no `series` exists, create a 1-game `casual` series automatically.

## 13c. Variant-name → category heuristic

Build a small lookup table mapping variant-name patterns (regex or substring match) to suggested categories — `Slayer`/`CTF` (default Halo variants) → `casual`; `Tournament` / `MLG` / `Comp` substring → `competitive`; explicit override flag from the M14 series setup beats the heuristic. Heuristic populates the suggested category on game creation; admin can re-categorize anytime through PB admin UI.

## 13d. Replace silent-drop queue (legacy bug)

Port `internal/pb/client.go` queue from legacy with one of:

- (a) Retry with exponential backoff.
- (b) Disk-spool overflow.

Decide during port; comment the tradeoff.

Smoke test: run a Halo: CE Slayer game start-to-finish on a single container → one `series` (category `casual`) + one `games` row + N `game_players` rows + event stream all land. Re-run with variant name "MLG Tournament v7" → category auto-suggested as `competitive`.
