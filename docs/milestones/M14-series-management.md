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
