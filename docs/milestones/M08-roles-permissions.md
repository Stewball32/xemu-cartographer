# Milestone 8 — Roles + permissions

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
