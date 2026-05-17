# Milestone 7 — Identity schemas: gamertags + teams

> Foundation for the match-aware kiosk (M9) and persistence stack (M13+). Many real users carry multiple gamertags ("Stewball32" / "Stewball" / "Stewie"); some rotate teams across events. Model this directly rather than forcing a 1:1 user↔gamertag↔team flattening.

## 7a. Schema design

Target shape:

- `users` (existing) — gains `default_gamertag` FK (nullable) so the system has a sensible "show me as" pick when one is needed and the user hasn't otherwise specified.
- `gamertags` (`user`, `tag`) — one row per (user, gamertag-string) combo. Simple two-column join. Optional fields to consider: `xbox_machine_name?` (if a tag is tied to a specific console), `notes?`. The `default_gamertag` FK on `users` lives there rather than as an `is_primary` flag on `gamertags` so there's a single canonical "default per user" with no risk of zero or multiple primaries; admins can change it via the user record.
- `teams` (`name`, `slug`, `created_by`).
- `trosters` — `team`, `gamertag`, `is_captain`, `is_manager`, `joined_at`, `is_active`. Both `is_captain` and `is_manager` are independent booleans (a player can be both, either, or neither). The roster row joins on `gamertag` (not `user`) so one user can rep different teams under different handles.

Decisions to lock during 7a: cascade rules on user deletion (soft-delete preferred so historical roster + game records survive), uniqueness constraint on `tag` (per-user unique; globally non-unique because two users can validly use the same handle on different consoles), whether `xbox_machine_name` belongs on `gamertags` or its own join table (recommend on `gamertags` until a real second-console-per-tag use case appears).

## 7b. PocketBase collections + admin UI

Add collections under [internal/pocketbase/schema/](../internal/pocketbase/schema/), one file each (`gamertags.go`, `teams.go`, `trosters.go`). Update the existing users schema to add `default_gamertag`. Build a SvelteKit admin/self-service UI under [sveltekit/src/routes/admin/identity/](../sveltekit/src/routes/admin/identity/) (or similar) for CRUD on gamertags + teams + trosters. Enforce row-level rules in PB API rules: a user can manage only their own gamertags + their own `default_gamertag` pick; team captains/managers can manage their team's trosters; everyone can read non-sensitive fields.

## 7c. Identity exposure to scraper + WS layers

Extend [routes/me.go](../internal/pocketbase/routes/me.go) (per the M4 pattern) to include the caller's gamertag list + default. Surface a `gamertag` lookup helper through `guards.Services` (probably via a new `internal/guards/interfaces/identity/` aggregate) so handlers can answer "is this gamertag X owned by user Y?" without circular imports. No backend persistence of in-game events yet — that's M13.

Smoke test: create user → add 3 gamertags → set one as default → create 2 teams → attach different gamertags to each team with captain/manager flags set → confirm `/api/me` returns the membership graph; admin UI round-trips create/edit/delete; non-admin user can't touch other users' gamertags; a non-captain/manager can't edit their team's roster.
