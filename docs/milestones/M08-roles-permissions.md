# Milestone 8 — Roles + permissions

> **Status:** In progress
> **Started:** 2026-06-03
> **Depends on:** M07 (identity schemas), M22 (audit_log foundation), M23 (notifications surface)

> Today `users.isAdmin` is a single boolean ([internal/pocketbase/schema/users.go:53-58](../internal/pocketbase/schema/users.go#L53-L58)) — fine for "is this person staff" but not enough as the surface area grows (tournament organizers, content moderators, stat reviewers, guild bot operators, etc.). Add a roles collection so permissions can be granted in named bundles, retire the bare `isAdmin` flag in favor of a role-membership check, and update the guard layer to consume roles instead of the boolean.

## 8a. Schema

New collections:

- `roles` (`slug`, `label`, `description`, `level: int`) — examples: `superuser`, `admin`, `tournament_organizer`, `team_manager`, `content_moderator`, `stat_reviewer`. `level` gives a coarse comparable rank for "is X at least an admin?" checks; finer-grained checks use slug membership.
- `user_roles` (`user`, `role`, `granted_by`, `granted_at`) — join table; a user can hold multiple roles.
- `permissions` (`slug`, `description`) and `role_permissions` (`role`, `permission`) — optional, for finer-grained "can_create_tournament", "can_post_to_discord_channel" style checks. Decide during 8a whether v1 ships with permissions tables or just role slugs (recommend role-slugs only for v1; defer permissions tables until a real use case demands them).

## 8b. Migrate `isAdmin`

Backfill a `roles` seed (`superuser` `level=100`, `admin` `level=50`, `member` `level=10`); migrate every existing `users.isAdmin=true` user into a `user_roles` row pointing at `admin`. Drop `users.isAdmin` from the schema and any code that reads it.

## 8c. Guard layer update

Replace [internal/pocketbase/routes/middleware/admin.go](../internal/pocketbase/routes/middleware/admin.go) `RequireAdmin()` with `RequireRole("admin")` (or `RequireMinLevel(50)`). Add a new `RequireRole(slug)` / `RequireAnyRole(slug...)` helper. Update every call site in `internal/guards/`, `internal/pocketbase/routes/`, `internal/websocket/handlers/`, etc. The frontend mirror in [sveltekit/src/lib/utils/guards.ts](../sveltekit/src/lib/utils/guards.ts) likewise switches from `isAdmin` boolean to a roles-array check; [auth.svelte.ts](../sveltekit/src/lib/stores/auth.svelte.ts) hydrates `roles: string[]` from `/api/me` instead of `isAdmin: boolean`.

## 8d. Self-service + admin UI

Admin page at [sveltekit/src/routes/admin/roles/](../sveltekit/src/routes/admin/roles/) for managing role definitions and assignments. Users see their own roles on their profile page (M15 will surface this).

Smoke test: log in as a pre-migration admin → user record now shows `roles: ["admin"]`, `isAdmin` field gone; admin UI works exactly as before. Create a `tournament_organizer` role, grant to a non-admin user → confirm M16 tournament-create routes accept the request when wired (or at least confirm the guard plumbing accepts the role check).

## Log

_Append-only. Never edit past entries; add a new dated line._

- 2026-06-03: M8 opened on `wip/milestone-8`. Locked scope decisions before substages started: (a) one branch + one PR at milestone close (M07/M22/M23 convention); (b) skip the `permissions` + `role_permissions` tables in v1 — role slugs only; (c) ship user ban + timeout in M08 (new `is_banned` + `banned_until` fields on users, AuthRule gating, `ActionBan`/`ActionTimeout` audit actions); (d) new `users_default_role` hook auto-creates a `member` `user_roles` row on user create, mirroring `users_default_gamertag`. Substage map: `8-pre` (PB rule spike, throwaway) → `8a` (schema + audit action constants, no behavior change) → `8b` (internal/roles helper + default-role hook + migration backfill, isAdmin column stays) → `8c` (swap 8 PB rules to subqueries) → `8d` (Go cutover + drop isAdmin field) → `8e` (frontend cutover + typegen) → `8f` (ban + timeout) → `8g` (admin UI at /admin/roles/) → `8z` (close-out). Dual-world checkpoint between 8b and 8d makes 8c (the risky PB rule cutover) independently revertible.
- 2026-06-03: 8-pre — PB rule spike PASS, Plan A locked. Ran against `task dev` on port 8091 with throwaway `_smoke_roles` / `_smoke_user_roles` / `_smoke_target` collections. Two rule shapes validated end-to-end:
  - **Subquery ListRule** `@collection._smoke_user_roles.user ?= @request.auth.id && @collection._smoke_user_roles.role.slug ?= "admin"` — PB accepts at save, admin sees rows, non-admin sees 0 rows, deleting admin's user_role row causes the next admin read to also return 0 rows (proves the subquery reads live, not a cached projection). The `?=` (any-match) operator is required because `@collection.X` returns a list.
  - **@now AuthRule** `banned_until = '' || banned_until < @now` on a throwaway `_smoke_auth_users` auth collection — PB accepts at save, future-banned user → HTTP 403 ("doesn't satisfy the collection requirements to authenticate"), past-banned user → HTTP 200 (passive expiry works), no-ban user → HTTP 200. Means 8f can ship the AuthRule extension without a hook-based fallback.

  Plan B (denormalized `is_admin` projection) and the OnRecordAuthRequest hook fallback for ban-gating are both dropped. ADR-0003 is not needed.
- 2026-06-03: 8a — schema + audit-action scaffolding (no behavior change). New `internal/pocketbase/schema/roles.go` registers the roles collection (slug + label + description + level 0–100, unique idx on slug) and seeds the three baseline rows (`superuser`/100, `admin`/50, `member`/10) inline at first-create — this is required data, not dev seed, so production deployments also boot with the slugs the guard layer needs. New `internal/pocketbase/schema/user_roles.go` registers the join collection (`user → users`, `role → roles`, `granted_by → users` nullable, `granted_at` autodate) with a unique `(user, role)` index + lookup `(user)` index. Both collections register through `identity.go` ahead of gamertags (chain reorder: `roles → user_roles → gamertags → teams → rosters → team_log → team_membership_requests`) so the M08b backfill in users.go can resolve `roles` at runtime. PB rules ship the pre-M08 isAdmin gate (`@request.auth.isAdmin = true`) — the 8c rewrite swaps them to the user_roles subquery. New audit-action constants land: `ActionRoleGrant` + `RoleGrantPayload{RoleSlug, ByMigration}`, `ActionRoleRevoke` + `RoleRevokePayload{RoleSlug, Reason}`, `ActionBan` + `BanPayload{Reason, ByMigration}`, `ActionTimeout` + `TimeoutPayload{Reason, ExpiresAt}`, `ActionUnban` + `UnbanPayload{Reason}`. `users.go` appends `is_banned` (hidden bool) + `banned_until` (date) fields under the soft-delete block; the AuthRule extension waits for 8f. `task typegen` produced `RolesRecord` + `UserRolesRecord` + `is_banned?` / `banned_until?` on `UsersRecord` (regenerated in this same commit since the 8e cutover will run another regen anyway). End-to-end verified live: API hits at `/api/collections/roles` + `/api/collections/user_roles` return the new shapes; `GET /api/collections/roles/records?sort=level` returns the three baseline rows; `users.is_banned` and `users.banned_until` show on `/api/collections/users`.
