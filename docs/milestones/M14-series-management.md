# Milestone 14 — Series management: setup, pick/ban, in-progress UI

> M13 lets the system *record* games as they happen; M14 lets users *intentionally set up* a multi-game series before play, optionally with a pick/ban round, and display the series in progress. Pick/ban is opt-in: if a series is set up with pick/ban, maps are committed up-front; otherwise the series just records whatever's played, game by game.

## 14a. Series creation UI

New page at `/series/new/`. Pick format (single, exact-N, best-of-N, first-to-X), participants (one or more teams from M7, or ad-hoc gamertags), category override, optional name. Creates a `series` row in the not-started state.

## 14b. Pick/ban round (optional)

When series creator opts in: present a draft-style map list, alternate ban / pick between participating teams, store the resulting map order on the series. UI flow can run synchronously in the browser or async via PB realtime — decide during 14b.

## 14c. Series-in-progress UI

New page at `/series/[id]/`. Shows series header (format, participants, category), per-game scoreboard (one row per played game), current standing (X-Y in a best-of-5), next map (if pick/ban committed) or "TBD". Auto-updates via PB realtime as new `games` rows are written by M13b.

## 14d. Series-aware game-end wiring

Extend M13b: when a game finishes and the host container is running under an active series (matched by container or gamertag), attach the new `games` row to that series instead of auto-creating a casual one. Series ends when format completion is reached (e.g. one team has won 3 of 5).

Smoke test: create best-of-3 series with 2 teams + pick/ban → 3 maps committed. Play 2 games (one team wins both). Series UI shows 2-0, marks series complete, doesn't accept the 3rd map. Compare to a casual no-pick/ban series of "first-to-2": same termination behavior driven by the format field.

## Log

_Append-only. Never edit past entries; add a new dated line._

- 2026-06-18: First increment — the **series-format termination logic** (14d core), the pure decision the rest of M14 hangs off. Implemented + unit-tested during the autonomous overnight run; the UI flows (14a setup, 14b pick/ban draft, 14c in-progress page) and the live game-end re-attachment wiring are deferred (need a live game + UI to verify).
  - New `internal/series` package: `Progress(format, targetN, teamWins) Standing` — decides whether a series is complete + who won, for `single` / `exact-n` / `best-of-n` (majority clinch) / `first-to-x`. Pure, no PB/IO; handles >2 participants (round-robin), ties under exact-n (no winner), and degenerate configs (clamps so a bad series still terminates). Unit-tested across all formats + edge cases.
  - Widened the M13 `series.format` SelectField to the full set (`single` / `exact-n` / `best-of-n` / `first-to-x`) so the schema, the format constants, and the logic agree.
  - **Deferred:** 14a `/series/new/` setup page; 14b pick/ban draft (interactive — the draft *rules* could later move into `internal/series` as a tested state machine, but the flow itself is UI); 14c `/series/[id]/` in-progress page (PB-realtime UI); 14d's live wiring (extend `internal/games.PersistFinishedGame` to attach a finished game to an active series via container/gamertag match, then call `series.Progress` to auto-end it). The `series.Progress` core is exactly what that wiring will call.
