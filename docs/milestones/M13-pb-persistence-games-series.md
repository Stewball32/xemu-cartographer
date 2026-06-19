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

- 2026-06-18: **`game_events` fork resolved — Stewart picked option (a)** ("match matters more than machine"), and the full game-end chain is wired. This closes the M13 decision; what's left is live verification + 13d's durable queue.
  - **Option (a) — `game_events` gains an optional `game` relation.** `schema/game_events.go` keeps the instance-keyed firehose shape (`instance`/`type`/`seq`/`tick`/`ts`/`data`) and adds a nullable `game` FK + `idx_game_events_game` index; it now registers through `identity.go` phase 4 (after `games`, since it relates to it) and reconciles the new column onto an existing collection. The live capture sink is untouched — events are written instance-keyed and stamped with their game at game-end.
  - **Backfill** (`internal/games.stampGameEvents`, called by `PersistFinishedGame`): stamps the game id onto that instance's still-unstamped (`game = ''`) `game_events` rows inside the finished game's time window (by `ts`). Idempotent (already-stamped rows are skipped via the `game=''` filter), so prior games' events aren't disturbed and a re-run stamps nothing. Integration-tested: in-window events get the id, before/after-window + other-instance + already-stamped don't; a second persist stamps 0.
  - **Chain wired** (`PersistFinishedGame` now returns `EventsStamped` + `SeriesStanding` + `RatingsUpdated`): game-end → stamp events → **M14** `series.Progress` (`advanceSeries` tallies the series' games' `winner_team`, stamps `ended_at` on completion) → **M15** stats become queryable from the persisted rows (no recompute step — `internal/stats` is query-time) → **M18** `rating.Update` (`updateRatings` applies a per-game-type, two-team-average Elo into the new `ratings` collection; FFA bumps game count only). Integration-tested end to end: a best-of-3 completes at 2-0 with `ended_at` set; winner's Elo rises, loser's falls, zero-sum, game counts increment.
  - **Production trigger** (`internal/scraper/manager/games_persist.go`): `runLive` defers `persistFinishedGame(svc)` after `captureLiveAsPrevious` (LIFO), mapping `cache.PreviousGame.GameData` → `games.FinishedGame` and calling the chain on a best-effort goroutine. **LIVE GAP (flagged, not blocking):** the `GameData → FinishedGame` projection + the trigger firing can only be verified against a real Halo: CE match — `internal/games` is fully unit-tested against the same `FinishedGame` shape, but the live mapping awaits xemu.
  - **13a retention decision:** **keep `game_events` full, no automatic pruning.** Rationale: events are small, full history powers replay/audit, and the per-game `game` FK makes targeted deletion cheap later. Roll-up (collapse to `game_players` aggregates) + prune-after-N-days is the documented follow-up if volume becomes a problem; `snapshot_blob` format stays open.
  - **Still open:** the live end-to-end verification (above), 13d durable queue (the writer is best-effort-with-error-return today), a per-game start timestamp (so the event window can also exclude idle/menu events between games — today idempotency scopes it to the current game's unstamped events), and the M15 `Won` / `winner_team` team-0-vs-no-winner ambiguity (a `has_winner`/draw flag).
