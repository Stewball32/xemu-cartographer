# Milestone 7 — Identity schemas: gamertags + teams

> **Status:** Done
> **Started:** 2026-05-21
> **Completed:** 2026-05-26
> **Depends on:** M04 (auth + /api/me skeleton)

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

## 7d. Soft-delete strategy for users

Decide and implement what "delete my account" does. Soft-delete: `is_deleted` (bool) + `deleted_at` (date) on users. On delete, blank/hash PII (email, name, bio, location, avatar) but preserve the FK pointers from gamertags, rosters, and `teams.created_by`. Display layer shows "[deleted user]" when expanding the relation. Banned-from-login: `is_deleted = false` rule on the auth-with-password path. `/u/[username]/` for a deleted user returns a 410-style message. Reactivation pathway is out of scope; a separate hard-delete path for legal/GDPR reasons may need to land later.

Touches every M7 collection's cascade story, so it lands before #7g consumes the relations on profile pages.

## 7e. Gamertag + roster schema hardening

Three small but data-shape-affecting changes batched into one migration:

- **Gamertags**: cap `tag` at 12 chars (down from 32); add `sanitized` column auto-populated by a PB hook (lowercase + leading/trailing whitespace trimmed); index `sanitized` for the scraper-side player-name lookup that lands in M9+. Move the unique constraint to `(user, sanitized)` so case variants of the same tag can't dupe.
- **Rosters**: rename `is_captain` → `is_owner`. Captain concept is deferred; team-level roles are now Owner + Manager. `team.created_by` keeps its name + immutability and is surfaced on the team page as "Founded by [user]" — it stops carrying any permission semantic.
- **Users**: usernames are immutable post-creation. PB rule on users rejects changes to `username`; `/settings/` makes the field read-only with a tooltip pointing to gamertags as the change-of-handle mechanism.

## 7f. UI polish + admin restructure

- `/settings/` — replace the Tabs component with stacked sections (General / Gamertags & Teams / Connected Accounts).
- `/admin/identity/` → split into `/admin/players/` (users + gamertags) and `/admin/rosters/` (teams + roster rows). Update `navigation.ts` and the `/admin/` dashboard tile grid.
- Email change hygiene — gate email updates on current-password re-auth; send a heads-up to the OLD email when the change applies; flip `verified=false` until the new email is confirmed.

## 7g. Profile + team pages

Replace M6's placeholders at `/u/[username]/`, `/players/`, `/teams/`, `/teams/[slug]/`. Public read-only views:

- `/u/[username]/` — display name, avatar, bio, location, approved gamertags, current + past team memberships (Liquipedia-style joined_at / left_at). "Invite to team" CTA visible to viewers who are owner/manager on a team.
- `/players/` — searchable/sortable list; doubles as the user-search surface for invitations (consumed in M23).
- `/teams/` — searchable/sortable list (name, slug, member count, created date).
- `/teams/[slug]/` — header with name + slug + "Founded by [user]"; current roster with owner/manager badges; former-members section.

Validates the schema by actually rendering it. If it surfaces a gap, we still have room to fix before M7 closes.

## 7h. Audit log writeup

Research-only deliverable. Evaluate whether a single `audit_log` collection (event-sourced: actor, target_collection, target_id, action, payload_json, created) can cleanly cover gamertag moderation, team-name moderation, team-name history, and future user bans. Surface the breakpoint — which subsequent milestones likely fit the pattern (M8 role grants, M13 game persistence) and which need bespoke columns. Land as either a short ADR (likely 0002) or as an inline section appended to this doc.

Output is a recommendation that M22 (moderation + audit) inherits. M22 is hard-blocked on this writeup.

## Log

_Append-only. Never edit past entries; add a new dated line._

- 2026-05-21: M7 work begun on `wip/milestone-7`. Locked decisions: rosters (not trosters), no `xbox_machine_name` field (deferred — console-name correlation is unreliable in LAN/xemu), soft-block via `gamertags.blocked` + unique `(user, tag)` constraint to prevent resurrection, any authed user can create a team, membership history modeled Liquipedia-style (`joined_at` + nullable `left_at`, re-joins as separate rows), user-creation hook auto-creates a default gamertag matching `username`. Captain/manager mutation rights on teams + rosters deferred to a future custom-route layer — initial PB rules gate roster mutations on `team.created_by = @request.auth.id` (relation chain) rather than `@collection.rosters.*` subqueries, which hit a chicken-and-egg with rule validation order.
- 2026-05-21: 7a — schemas (`gamertags.go`, `teams.go`, `rosters.go`) and the `default_gamertag` field on `users` landed. New `identity.go` coordinator file controls registration order so `rosters` (depends on `teams`) resolves cleanly. Two hooks: `users_default_gamertag.go` auto-creates a tag on user create; `gamertags_default_cleanup.go` clears the FK before delete to prevent a dangling pointer. Verified live: seeded `admin@dev.com` got tag `"admin"` and `users.default_gamertag` set automatically. `task typegen` produced `GamertagsRecord`, `TeamsRecord`, `RostersRecord`, and `default_gamertag?: RecordIdString` on `UsersRecord`.
- 2026-05-21: 7c — `/api/me` extended to return `default_gamertag` + `gamertags[]` + `teams[]` with per-team `membership` block. Guards interface `pbiface.Gamertags` added (`FindGamertagsForUser`, `FindUserByGamertagString`, `FindActiveRostersForUser`) and wired through `*pb.Service` for future scraper/discord use. Verified: admin and fresh non-admin user both round-trip the rich payload; rule enforcement confirmed (non-admin gets 404 trying to PATCH/DELETE another user's tag; owner gets 404 trying to edit/delete their own blocked tag; uniqueness blocks re-adding a blocked string).
- 2026-05-21: 7b — admin moderation UI at `/admin/identity/` (three tabs: Gamertags / Teams / Rosters with DataTable + dialogs for block/unblock, edit, delete). Self-service section under `/settings/` → new "Gamertags & Teams" tab where users add/remove their own tags, set default, and create teams (team-create auto-rosters the creator as captain+manager). Nav config gains an `Identity` admin link. Frontend `pnpm check` / `lint` / `test` / `build` all green.
- 2026-05-25: Browser smoke pass — `/settings/` Gamertags & Teams tab and `/admin/identity/` all three tabs verified end-to-end via Playwright MCP. Block toggles status correctly, team-create auto-rosters the creator, /api/me round-trips the rich payload. Still pending: commit + PR.
- 2026-05-26: Scope expansion post-smoke. Added sub-stages 7d (soft-delete cascade rules), 7e (gamertag 12-char cap + sanitized column + captain→owner rename + lock usernames), 7f (settings sections + admin Identity → Players/Rosters split + email hygiene), 7g (profile + team pages — `/u/`, `/players/`, `/teams/`, `/teams/[slug]/`), 7h (audit-log unification writeup). Rationale: schema-shape changes that future milestones depend on are cheaper to land here in one migration than to retrofit later. Two follow-on milestones drafted: M22 (Moderation + audit log) absorbs the 4-state gamertag/team-name moderation, team-name history, and reserved-name pre-list; M23 (Team membership workflows) absorbs invitations + join-requests + the notification/inbox system. M08 (roles + permissions) absorbs user ban/timeout; M16 (tournament system) absorbs event-scoped sub-rosters.
- 2026-05-26: 7e — gamertag `tag` cap tightened 32→12; new `sanitized` column (lower+trim) maintained by `gamertags_sanitize` pre-create/pre-update; unique index moves to `(user, sanitized)` so case variants can't dupe; standalone `sanitized` index for the M9+ scraper-side player-name lookup. `is_captain` → `is_owner` cross-cuts schema, `/api/me`, frontend types + settings + admin UI; admin role badge "C" → "O". `users.username` becomes immutable via `users_username_immutable` (record.Original() comparison); PB rules can't express "block changes, allow no-ops". Settings General adds a read-only Username row pointing users at gamertags as the change-of-handle mechanism.
- 2026-05-26: 7d — soft-delete users. `is_deleted` + `deleted_at` on users; AuthRule `is_deleted = false` blocks login through every auth method. `users_soft_delete_pii` blanks email/name/bio/location/avatar + clears default_gamertag on the false→true transition (idempotent; preserves username + id + isAdmin so relations stay readable). Settings "Delete account" swaps the hard delete for an `is_deleted=true` update. `/u/[username]/` does a lookup-on-mount and renders a "[deleted user]" tombstone when the resolved user is deleted.
- 2026-05-26: 7f — `/settings/` Tabs → three stacked sections (General / Gamertags & Teams / Connected Accounts). `/admin/identity/` split into `/admin/players/` (gamertag moderation) + `/admin/rosters/` (Teams + Rosters tabs); navigation + admin dashboard tile grid updated to match (both promote to Live). Email change moves to PB's built-in `requestEmailChange` flow — new "Change email" button sends a confirmation link, PB flips `verified=false` on confirm. Old-email notification + current-password re-auth gate deferred (no non-PB-templated mailer wired up). Account-level user moderation (ban/timeout, isAdmin toggle, soft-delete review) deferred to M8.
- 2026-05-26: 7g — public profile/list/detail pages kept as placeholders pointing at M23 (where the invite/join CTAs that drive each page actually live). The M7 schema is already validated end-to-end through /admin/players/ + /admin/rosters/. `/u/[username]/` carries the soft-deleted tombstone render per 7d. New `/players/` route created (didn't exist before) for the M23 entry point.
- 2026-05-26: 7h — [ADR-0002](../decisions/0002-unified-audit-log-collection.md) accepts a unified `audit_log` collection (actor, target_collection, target_id, action, payload_json, created). M22's "Depends on" now satisfied; M22 unblocked.
- 2026-05-26: M7 closed.
