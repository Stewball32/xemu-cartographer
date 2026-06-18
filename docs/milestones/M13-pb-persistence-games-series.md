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

## Log

_Append-only. Never edit past entries; add a new dated line._

- 2026-06-18: First increment — the **schema + persistence writer + heuristic** (13a partial + 13b + 13c), implemented and unit-tested during the autonomous overnight run. The Live→Ready manager wiring + queue robustness (13d) are deferred because they can only be verified by running a real game start-to-finish (no xemu here).
  - **13a (3 of 4 collections):** `series` (name?, format, target_n, category, created_by?, started_at, ended_at?, tournament?, tournament_round?), `games` (series FK, container, host_machine_name, map, gametype, variant_name, started_at, ended_at, winner_team?, score_summary, snapshot_blob?), `game_players` (game FK, gamertag, team, kills/deaths/assists/score, time_alive_ms, weapon_loadout?). Authed read, nil mutate (writes flow through the writer / future M14 routes via `app.Save`). The FK chain (series → games → game_players) is ordered through `identity.go` phase 4 because per-file `init()`s run alphabetically — the reverse of the dependency order.
  - **13c:** `internal/games.SuggestCategory(variantName)` — pure, tested. Competitive markers (`tournament`/`mlg`/`comp`/`hcs`/`pro`) → `competitive`; else `casual`. The `tournament` category is set explicitly by M14, not inferred.
  - **13b:** `internal/games.PersistFinishedGame(app, FinishedGame)` — writes a `games` row + N `game_players`, auto-creating a 1-game series (category from the heuristic) when no SeriesID is supplied. Unit-tested against a `tests.NewTestApp()` (auto-create path categorizes "MLG Tournament v7" → competitive; existing-series path adds no extra series).
  - **DEFERRED + DECISION NEEDED — the 4th collection, `game_events`.** A `game_events` collection **already exists**: the M5 capture-sink firehose keyed by `instance` (`{instance, type, seq, tick, ts, data}`), written by the `pb:game_events` sink. M13's spec wants a `game`-FK'd append-only per-game event log under the same name — a **direct collision**. I did **not** create/alter it, to avoid clobbering the live capture path. **Stewart's call:** (a) extend the existing `game_events` with an optional `game` relation and back-fill it at game-end by instance+tick range; (b) add a separate game-keyed collection under a new name (e.g. `game_event_log`); or (c) leave events in the instance-keyed firehose and have stats/replay join by instance + `[started_at, ended_at]`. My lean is (a) — least duplication, keeps one event table.
  - **Also deferred:** 13b's wiring into the scraper's Live→Ready transition (best-effort goroutine calling `PersistFinishedGame` — needs a live game to verify), and 13d queue robustness (retry/backoff vs disk-spool — the writer currently returns an error for a best-effort caller to log; a durable queue is the follow-up). `snapshot_blob` format (full vs trimmed instanceCache) and `game_events` retention are still open per 13a.
