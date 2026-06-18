# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

> See also: [README.md](README.md) for project overview, tech stack, architecture diagram, and quick-start guide.

## Reference docs

Before writing or reviewing code that touches a third-party library where the API may have drifted from your training data, consult up-to-date docs rather than guessing.

- **Skeleton UI v4** — [sveltekit/docs/skeleton-llms.txt](sveltekit/docs/skeleton-llms.txt) is a table of contents of Skeleton's official docs (components, theming, Tailwind v4 integration). Read it first to locate the right page, then WebFetch the specific page under `https://www.skeleton.dev/` (e.g. `https://www.skeleton.dev/docs/svelte/framework-components/app-bar.md`, `https://www.skeleton.dev/docs/svelte/tailwind-components/buttons`). Always use the **Svelte** section, not React.
- **SvelteKit, PocketBase JS SDK, Disgo, Tailwind v4** — WebFetch the official docs site (`kit.svelte.dev`, `pocketbase.io/docs`, `disgo.dev`, `tailwindcss.com`) rather than inventing an API.

## Project meta-docs

[`docs/README.md`](docs/README.md) is the source of truth for the project's meta-docs convention. Follow it when adding planning, status, or decision documents.

- New milestone → copy [`docs/milestones/_template.md`](docs/milestones/_template.md) to `docs/milestones/M??-kebab-name.md`, then add a row to `docs/milestones/README.md`.
- New decision → copy [`docs/decisions/_template.md`](docs/decisions/_template.md) to `docs/decisions/????-kebab-name.md`, then add a row to `docs/decisions/README.md`. ADRs are immutable once Accepted — supersede with a new ADR rather than editing.
- Update [`docs/STATUS.md`](docs/STATUS.md) whenever the "Now" set of work changes.
- Add user-visible changes to `CHANGELOG.md` under `[Unreleased]`; cut a versioned section when shipping (SemVer, Keep-a-Changelog format).
- Dates are always absolute (`YYYY-MM-DD`). Milestone `Log` sections and the ADR index are append-only.

## Development Commands

```sh

# Install task runner and hot reload (to /usr/local/bin so both `task dev` and `sudo task dev` work; see README Prerequisites for the rationale and opt-out)

sudo env GOBIN=/usr/local/bin go install github.com/go-task/task/v3/cmd/task@latest
sudo env GOBIN=/usr/local/bin go install github.com/air-verse/air@latest

# Run both backend and frontend dev servers

task dev

# Backend only (hot reload)

task dev:backend

# Frontend only (run from sveltekit/)

task dev:frontend

# Build for production

task build

# Build and run container

task container:build
task container:run

# Clean build artifacts

task clean

# Run server directly (no Task/Air)

go run ./cmd/server serve
./bin/server serve

# Backend tests (Go) — local-only; CI runs `go vet` + build, not `go test`

go test ./...
go test ./internal/scraper/manager/...   # single package
go vet ./...

# Frontend type-check, lint, format (run from sveltekit/)

cd sveltekit
pnpm check          # svelte-check + TypeScript
pnpm lint           # prettier + eslint
pnpm format         # prettier --write

# Frontend tests (run from sveltekit/)

pnpm test           # vitest run — unit tests
pnpm test:watch     # vitest watch
pnpm test:e2e       # playwright — e2e tests in sveltekit/e2e/

# Generate PocketBase TypeScript types (requires running dev server)

task typegen
```

## Architecture

Single Go binary (`cmd/server`) runs four concurrent systems plus an optional fifth:

1. **PocketBase** — REST API, auth (JWT), SQLite database, static file server (serves `pb_public/`), uses `net/http.ServeMux` router
2. **Disgo Discord bot** — connects via gateway in PocketBase's OnServe hook, non-blocking
3. **WebSocket handler** (`coder/websocket`) — custom route on PocketBase's router with optional JWT auth, Hub for managing clients/rooms/broadcasting
4. **Scraper manager** (`internal/scraper/manager`) — owns per-xemu-instance memory-scraper goroutines; broadcasts `current_state` / `state_update` / `event` envelopes to per-instance `host:<name>` rooms and a cross-instance `host:all` summary room (M5 stages 5c–5e — see `envelopeType*` constants in [internal/scraper/manager/loop.go](internal/scraper/manager/loop.go); on-demand event log via the `request_events` WS handler + `EventsReply()`; system-snapshot identity sourced from [internal/scraper/xbox/](internal/scraper/xbox/))
5. **Podman containers + discovery watcher** (optional, gated by `CONTAINERS_ENABLED=true`) — `internal/podman` shells out to provision xemu+browser pairs; `internal/discovery` polls the QMP socket dir and auto-Start/Stops scrapers as instances appear/disappear

The SvelteKit frontend is built with `@sveltejs/adapter-static` into `pb_public/`, which PocketBase serves automatically. The `fallback: 'index.html'` config enables SPA-style client-side routing.

Protected pages can be served through custom PocketBase routes that validate JWT auth before serving the static file, while public pages are served directly from `pb_public/`. The kiosk noVNC UI is fronted by a same-origin reverse proxy (`/api/admin/containers/{name}/kiosk/...`, see [internal/pocketbase/routes/containers/proxy.go](internal/pocketbase/routes/containers/proxy.go)) so only PocketBase's port needs to be public — per-container ports stay bound to 127.0.0.1. M09 widens the proxy + VNC-relay auth from admin-only to `authorizeKioskAccess` ([internal/pocketbase/routes/containers/auth.go](internal/pocketbase/routes/containers/auth.go)): admins reach any container, and a non-admin reaches the one container their gamertag is currently rostered in (re-checked per request), which is what lets the player-facing `/play/` page embed the kiosk + controller.

## Backend Structure

### Startup sequence (`cmd/server/main.go`)

1. Create PocketBase app instance
2. Register record lifecycle hooks — `hooks.RegisterAll(app)` (callback registration, fires later)
3. In OnServe hook:
   - Register collection schemas (`schema.RegisterAll(app)`)
   - Register OAuth2 providers (`oauth.RegisterAll(app)`) — must run after schema
   - Run `seed.Run(app)` (no-op in production builds), then `seed.RegisterContainerSnapshotHooks` (mounted _after_ seeding so the seeder's writes don't trip the snapshot)
   - Build `*Services` skeleton with `App` + `PB` populated; later subsystems mutate the same pointer as they come up (no `SetServices` callback needed for systems that already hold the pointer)
   - Construct scraper `Manager`, attach to `svc.Scraper`, hand it to the scraper route group
   - If `CONTAINERS_ENABLED=true`: build podman `Manager`, wire it into the containers route group, and (if `CONTAINERS_SOCKET_DIR` is set) start the discovery watcher with `onAdd → scrMgr.Start` and `onRemove → scrMgr.Stop`
   - Register custom API routes (`routes.RegisterAll(se)`)
   - Initialize WebSocket Hub, start its Run() goroutine, mount `/api/ws` endpoint, populate `svc.WS`
   - Start Disgo bot gateway connection (non-blocking); on success populate `svc.Discord`
   - Hand `*Services` to hooks + Disgo commands via `SetServices`
4. Register OnTerminate hook: cancel discovery watcher → stop scrapers (must precede hub shutdown so in-flight tick broadcasts don't write to a closing channel) → stop hub → close Disgo bot
5. Call `app.Start()` (blocking)

### Key packages

The README's [Project Structure](README.md#project-structure) tree covers the directory layout. The table below highlights packages with cross-cutting roles or non-obvious behavior.

| Package                                 | Purpose                                                                                                                                                                                                                                                          |
| --------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/guards`                       | Unified cross-system guards, `Services` struct, `GuardFunc` type                                                                                                                                                                                                 |
| `internal/guards/interfaces/`           | Per-method interfaces, one file each, grouped by system (`discord/`, `websocket/`, `pocketbase/`, `scraper/`). Compose into aggregate `Service` interfaces via embedding.                                                                                        |
| `internal/pocketbase`                   | PB service wrapper — implements `pbiface.Service`                                                                                                                                                                                                                |
| `internal/pocketbase/routes/containers` | `/api/admin/containers/*` CRUD + kiosk noVNC reverse proxy + VNC pass-through                                                                                                                                                                                    |
| `internal/pocketbase/routes/scraper`    | `/api/admin/scraper/*` — list/start/stop scraper runners (typed against `scraperiface.Service`, no compile-time dep on the manager)                                                                                                                              |
| `internal/pocketbase/routes/xemu`       | `/api/admin/xemu/*` — memory-bridge probe/smoke + live diagnostics (`probe.go`, `probe_title.go`, `sample_deltas.go`, `scan_string.go`)                                                                                                                          |
| `internal/pocketbase/routes/teams`      | M23c — `/api/teams/{teamId}/invite` (owner/manager → user) + `/api/teams/{teamId}/join-requests` (user → team). RequireAuth; per-handler guards via `internal/teamperms`.                                                                                                                                                                                                                                                                                                                                                  |
| `internal/pocketbase/routes/adminusers` | M08g — `/api/admin/users/*` admin moderation routes. RequireAuth + RequireAdmin. `POST /{id}/roles` + `DELETE /{id}/roles/{slug}` delegate to `internal/roles.Grant/Revoke`. `POST /{id}/{ban,unban,timeout}` `app.Save` the users record and emit the matching audit row directly (bypassing the `users_ban_transitions` hook to avoid double-audit) so the request body's `reason` lands on the audit log.                                                                                                                                                                                                                                                                                                                                                  |
| `internal/pocketbase/routes/team_membership_requests` | M23d — `/api/team-membership-requests/{id}/{accept,decline,cancel}`. Accept resolves the recipient's `default_gamertag` server-side and creates the roster row.                                                                                                                                                                                                                                                                                                                                |
| `internal/pocketbase/routes/rosters`    | M23d — `/api/rosters/{id}/{leave,remove}`. Self-leave stamps `left_at` quietly; owner/manager remove notifies the kicked user.                                                                                                                                                                                                                                                                                                                                                                                  |
| `internal/pocketbase/seed`              | In-process dev seeder — `seed.go` (`//go:build dev`) + `stub.go` (`//go:build !dev`) + `data.go` defines seed vars; `containers_snapshot.go` keeps a record-backed snapshot of live podman state                                                                 |
| `internal/websocket`                    | WebSocket Hub, client management, message routing, JWT auth upgrade                                                                                                                                                                                              |
| `internal/websocket/rooms`              | Room type definitions with guard lists — one per file (`overlay`, `admin`, `public`, `host:<name>`, `host:all`)                                                                                                                                                  |
| `internal/scraper`                      | `GameReader` interface + game registry; `Detect()` picks a plugin by xemu title ID                                                                                                                                                                               |
| `internal/scraper/manager`              | Per-instance scraper runner: opens `xemu.Instance`, runs phase-driven tick goroutine, broadcasts to `host:<name>` + `host:all` rooms; implements `scraperiface.Service`. M09 `Membership()` projects each runner's roster (console / machine / player names) into a `ContainerMembership` view for gamertag→container match routing.                                                                                          |
| `internal/scraper/haloce`               | Halo: CE `GameReader` (offsets, game-data / tick / event readers); self-registers via blank import in `main.go`                                                                                                                                                  |
| `internal/scraper/xbox`                 | Title-agnostic Xbox console primitives — one file per scan target: `xbe_certificate.go`, `console_name.go`, `serial.go`, `mac.go`, `clock.go`, `timezone.go`, `video_standard.go`; plus `memory.go` (kernel helpers), `offsets.go`, `encoding.go` (Xbox strings) |
| `internal/scraper/roster`               | M10d dummy-player / neutral-host filter. `FilterRoster(players, Config{IsNeutralHost, DummyGamertags})` is the pure, shared cleaning step that drops a neutral host's local out-of-bounds dummy + globally-allowlisted dummy gamertags from viewer-facing surfaces (overlays / minimaps / stats). Config sources: the `is_neutral_host` bool on the `containers` collection + the admin-gated `dummy_gamertags` collection. Raw debug views keep the unfiltered roster.                                                                                                                              |
| `internal/games`                        | M13 games-persistence path. `PersistFinishedGame(app, FinishedGame)` writes a `games` row + N `game_players` on a finished contest, auto-creating a 1-game `series` when none is supplied; `SuggestCategory(variantName)` is the 13c variant→category heuristic. Direct `core.App` calls (mirrors `internal/roles`/`teamperms`). The `series`/`games`/`game_players` schemas register through `identity.go` phase 4 (FK order). The Live→Ready wiring + the `game_events` linkage (collides with the M5 instance-keyed firehose) are deferred — see the M13 Log.                                                                                                                              |
| `internal/xemu`                         | xemu QMP client + memory bridge — `Instance.Init`, GVA→GPA translation, `ReadBytes`/`ReadAt`                                                                                                                                                                     |
| `internal/podman`                       | Podman CLI wrapper — container pair lifecycle, stride-wise port allocation, state tracking                                                                                                                                                                       |
| `internal/discovery`                    | Polling watcher over `CONTAINERS_SOCKET_DIR`; emits `onAdd(name, sock)` / `onRemove(name)` so the scraper manager can attach/detach as containers come and go                                                                                                    |
| `internal/audit`                        | Shared audit-log writer per [ADR-0002](docs/decisions/0002-unified-audit-log-collection.md). `Write(app, actor, action, target, payload)` + `WriteRef(...)` (post-delete / synthetic rows). One file per action constant — `action_<verb>.go` pairs `ActionXxx` with `XxxPayload`. M22 lands block / unblock / approve / edit / rename; M08 adds role_grant / role_revoke / ban / unban / timeout. `Write` skips the `actor` relation when the supplied record isn't from the `users` collection (PB superusers + nil actors record as `actor=null`).                                                                |
| `internal/roles`                        | M08 roles + user_roles read/write helpers. `Has(app, userID, slug)` / `Slugs(app, userID)` for reads; `Grant(app, userID, slug, grantedBy)` / `Revoke(app, userID, slug, by, reason)` pair the user_roles mutation with the matching `audit_log` row. `IsAdminAuth(app, auth)` is the superuser-OR-admin-role shorthand consumed by hooks + middleware. Mirrors the `internal/teamperms` pattern — no `Service` interface boundary since scraper + Discord never need role checks.                                                                                                                                                              |
| `internal/reservednames`                | M22e reserved-name pre-list. `Match(app, candidate)` does substring-on-lowercase against the `reserved_names` collection; consumed by the `gamertags_reserved_name` and `teams_reserved_name` hooks to flip new submissions to `status="pending"` before the status-transition hooks fire.                                                                                                          |
| `internal/notifications`                | M23a notification writer. `Notify(app, user, type, payload)` mirrors `audit.Write`. One file per type — `notification_<event>.go` pairs `NotifTypeXxx` with `XxxPayload`. Reads through the `notifications` collection per the M23a schema; rules gate ownership and the `notifications_field_lock` hook restricts non-admin updates to the `read` field.                                                                                                                                                       |
| `internal/teamlog`                      | M23c team activity log writer. `Write(app, team, actor, event, subjectUser, subjectGamertag, payload)` writes to the `team_log` collection. One file per event — `event_<verb>.go` pairs `EventXxx` with `XxxPayload`. Twin-write target for renames + status changes (the M22d `audit_log.ActionRename` row stays); canonical surface for membership churn so the public team page reads activity without admin-only audit_log access.                                                                                              |
| `internal/teamperms`                    | M23c cross-cutting team-permission helpers. `IsOwnerOrManager`, `HasActiveMembership`, `OwnersAndManagers` (notification fan-out target), `FindPendingRequest`. Direct `core.App` calls; no interface boundary — scraper + Discord don't need team perms. Consumed by `routes/teams/`, `routes/team_membership_requests/`, `routes/rosters/`, and the `users_soft_delete_pii` cascade.                                                                                                                              |
| `internal/gamertags`                    | M09 gamertag→roster match helper. `SanitizedForUser(app, userID)` returns a user's non-blocked, lowercased+trimmed gamertag strings (the `gamertags.sanitized` column) for testing against live container rosters. Leaf package (`core.App` + dbx) so its three consumers — the WS `join_room` guard, the kiosk/VNC proxy (`authorizeKioskAccess`), and the `/api/me/match` resolver — share one query path without importing each other.                                                                                                                              |

## Frontend Structure

- **UI framework:** Skeleton UI v4 (Svelte 5 + Tailwind CSS v4); six bundled themes in [sveltekit/src/lib/styles/](sveltekit/src/lib/styles/) (`theme-{default,forerunner,midnight,norcal,phosphor,xbox}.css`), one is selected via `data-theme="..."` on `<html>` in [sveltekit/src/app.html](sveltekit/src/app.html). Per-theme body decorations (e.g. xbox's warped hex mesh + jewel glow) live behind `[data-theme='...']` selectors in [sveltekit/src/routes/layout.css](sveltekit/src/routes/layout.css). Theme previews: `/debug/theme/` and `/debug/theme-compact/`.
- **API client:** PocketBase JS SDK (`pocketbase` npm package) — singleton in `src/lib/pocketbase.ts`; in dev points to `http://localhost:PORT`, in production passes `undefined` (same-origin relative)
- **Auth store:** `src/lib/stores/auth.svelte.ts` — uses Svelte 5 runes (`$state`/`$derived`), not writable stores
- **Mode store:** `src/lib/stores/mode.svelte.ts` — dark/light mode toggle, persisted in `localStorage`; call `mode.toggle()` or `mode.set('dark'|'light')`
- **Toaster:** `src/lib/stores/toaster.ts` — global Skeleton toast singleton (`toaster`); import and call `toaster.trigger(...)` from any component
- **Navigation:** `src/lib/config/navigation.ts` — central nav link config consumed by all four layout nav components; edit here to add/remove nav links
- **App config:** `src/lib/config/app.ts` — exports `APP_NAME` (displayed app name) and `OAUTH_PROVIDERS` (display labels + icons per provider); actual enabled providers are discovered at runtime from PocketBase's `listAuthMethods()` API
- **WebSocket:** Browser native `WebSocket` API connecting to `/api/ws?token=PB_JWT`
- **Routing:** SvelteKit file-based routing in `sveltekit/src/routes/`; `+layout.ts` sets `ssr = false`, `prerender = true`, `trailingSlash = 'always'` globally
- **Admin pages:** Under `sveltekit/src/routes/admin/`: `/admin/` dashboard, `/admin/pod/` listing + `/admin/pod/[name]/` per-pod hub (View / Debug / Probe sub-pages), `/admin/capture-policies/`, `/admin/players/` (gamertag 4-state moderation queue — M07/M22), `/admin/rosters/` (Teams + Rosters tabs, with the Teams tab carrying the same 4-state queue — M07/M22), `/admin/reserved-names/` (M22e pre-list curation). `/admin/games/` (M13) and `/admin/roles/` (M08) are placeholders. The route group is gated by `requireAdmin` hoisted into `routes/admin/+layout.ts`.
- **Player "my match" page:** [sveltekit/src/routes/play/](sveltekit/src/routes/play/) (M09, RequireAuth not admin) polls `GET /api/me/match` (server resolves the caller's gamertag against live container rosters — `host:summary` is admin-only so this can't be a client-side match) and renders [PlayKiosk](sveltekit/src/lib/components/play/PlayKiosk.svelte) (reuses `KioskFrame` + `XboxController` + `VNCKeyboard`) for the matched container, or an idle state. The pure resolve→phase logic lives in [sveltekit/src/lib/utils/play-match.ts](sveltekit/src/lib/utils/play-match.ts) (unit-tested).
- **Admin debug page:** [sveltekit/src/routes/admin/pod/[name]/debug/](sveltekit/src/routes/admin/pod/[name]/debug/) renders per-instance tabs (Overview / Game / Tick / Events / Probe / Raw JSON). Sub-components live in [sveltekit/src/lib/components/debug/](sveltekit/src/lib/components/debug/) (`OverviewCard`, `KvCard`, `ColGroupedTable`, `PlayerListItem`, `PlayerDetailPanel`). The Probe tab consumes `GameReader.BuildScoreProbe()` and `LastStateInputs()` from [internal/scraper/scraper.go](internal/scraper/scraper.go) — when adding diagnostics to a game plugin, surface them through these methods.
- **Tables:** use `<DataTable>` from [sveltekit/src/lib/components/ui/DataTable.svelte](sveltekit/src/lib/components/ui/DataTable.svelte) for any list of records. Conventions (required props, density, cell patterns, order stability) live in [sveltekit/docs/TABLES.md](sveltekit/docs/TABLES.md).
- **Nav layout & Skeleton rail-gap override:** [sveltekit/docs/NAV.md](sveltekit/docs/NAV.md) — Skeleton v4's rail mode stretches `Navigation.Menu` via `flex: 1` and centers items, producing a large gap between groups; the doc records the targeted override and the Tailwind v4 `!` suffix needed to win on specificity. Re-apply this override in any template-derived repo.
- **Build:** adapter-static outputs directly to `pb_public/` with SPA fallback
- **Env:** `vite.config.ts` uses `envDir: '..'` to read from root `.env` — no separate `sveltekit/.env`
- **Package manager:** pnpm

### Responsive layout

The root layout (`+layout.svelte`) implements a 3-mode navigation system driven by a single `NavPanel` component:

| Breakpoint       | Nav mode                                                             |
| ---------------- | -------------------------------------------------------------------- |
| Mobile (`< sm`)  | Bottom bar (`MobileNav`) + slide-in overlay drawer (`NavPanel`)      |
| Desktop (`< lg`) | Rail sidebar — icons only (`NavPanel layout="rail"`)                 |
| Desktop (`≥ lg`) | Toggle between rail and full sidebar via `NavToggle` in the `Header` |

`NavToggle` toggles `navOpen`, which controls both the desktop rail↔sidebar expansion and the mobile overlay open/close state. `NavPanel` derives its Skeleton `layout` prop (`"rail"` | `"sidebar"`) from `open` and `isDesktop`.

## Cross-System Architecture

The main systems (PocketBase, Disgo, WebSocket, Scraper) never import each other. Cross-system communication is mediated through:

1. **Interfaces** (`internal/guards/interfaces/`) — one interface per file, organized in per-system subdirectories (`discord/`, `websocket/`, `pocketbase/`, `scraper/`). Small interfaces compose into aggregate `Service` interfaces via embedding.
2. **Services struct** (`internal/guards/services.go`) — bundles all system references (`App`, `PB`, `WS`, `Discord`, `Scraper`). Fields are nil if the system is not running.
3. **Dependency injection** — `main.go` builds the `Services` struct early and either passes the pointer at construction time or calls `SetServices()` once subsystems come up. Subsystems that hold the `*Services` pointer (e.g. the scraper manager) see fields like `svc.WS` populate later in OnServe without needing a setter.

Handler flow: **Trigger → Resolve → Guard → Action**

- **Resolvers** stay in their own package (`pocketbase/resolvers/`, `disgo/resolvers/`, `websocket/resolvers/`) and only talk to their own system
- **Guards** (`internal/guards/`) take `*Services` and check cross-system permissions
- **Actions** are called through `Services` interfaces (e.g., `svc.Discord.SendNotification()`, `svc.WS.BroadcastRaw()`, `svc.Scraper.JoinReplayMessages()`)

Handlers orchestrate by calling resolvers/guards/actions from multiple systems — no resolver or guard calls sideways into another package's resolvers.

The scraper manager is special: it holds `*guards.Services` and broadcasts to per-instance `host:<name>` rooms (plus a cross-instance `host:all` summary room) via `svc.WS.BroadcastRawToRoom(...)`. Late-joining clients are caught up two ways: (1) `Manager.JoinReplayMessages()`, called from the `join_room` WS handler, replays the latest `current_state` envelope so the UI can render immediately; (2) `Manager.EventsReply()`, called from the `request_events` WS handler, returns the recent event log filtered by `since_tick` + `types`. Without join replay, a client that connects mid-match never gets map/players/power-item-spawn data; without events reply, on-demand event-log requests would have no path back through the import-cycle-free `scraperiface` boundary.

## Conventions

- **Adding new routes/hooks/commands/WS handlers:** create a new file in the relevant package, define a function, and call `register(fn)` from `init()`. No other file needs to change — the package-level `init()` runs automatically on import.
- PocketBase v0.36.7 — uses `net/http.ServeMux`, NOT Echo. Hooks use `OnServe` not `OnBeforeServe`.
- PocketBase extensions follow a registration pattern: hooks register before OnServe, schema/routes register inside OnServe via `RegisterAll()`
- One `.go` file per logical domain in `schema/`, `hooks/`, `routes/`, and `commands/`
- PB record hooks use `routine.FireAndForget` for async external calls (Discord API)
- Clone record data into local variables before goroutines — event objects are not concurrent-safe
- WebSocket auth: validate `?token=` query param, attach user if valid, allow anonymous if not
- WebSocket Hub supports: Broadcast (all clients), SendToUser (by user ID), SendToRoom (room members), plus `*Raw` variants taking `[]byte` for cross-system use via interfaces
- Disgo uses `discord.SlashCommandCreate` for slash commands, raw event listeners for gateway events
- Disgo actions take `*bot.Client` as first param — also exposed as methods on `Bot` for interface compliance
- Disgo components are pure builder functions (no registry, no init) — one file per button/embed/row
- Cross-system guards in `internal/guards/` take `*Services` + `*core.Record`, usable from any system — one `require_*.go` file per guard (see `require_admin.go`, `require_auth.go`, `require_connected.go`, etc.); compose them with `compose.go`
- **M08 admin gating:** PB collection rules use the `hasAdminRole` constant in [internal/pocketbase/schema/rules.go](internal/pocketbase/schema/rules.go) (`@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= "admin"`) — wrap in parens when composing with `||`/`&&`. Go middleware + hooks call `roles.IsAdminAuth(app, auth)` (superuser OR holds the admin role) from [internal/roles](internal/roles/roles.go). The pre-M08 `users.isAdmin` boolean is gone. Adding a new gated collection: register through `identity.go` AFTER `roles` + `user_roles` (PB validates rules at save time; alphabetical init order would put `audit_log` etc. before `user_roles` and crash boot). The `roles` + `user_roles` collections themselves use nil mutate rules because of the first-boot circular reference; admin mutations on them flow through the M08g custom routes under `/api/admin/users/*` (which `app.Save` to bypass rules).
- Interface files use one-interface-per-file convention for merge-safe parallel development
- Custom routes registered in OnServe take priority over pb_public/ static file serving
- `PUBLIC_PB_PORT` in root `.env` — single port variable used by Taskfile, compose, Containerfile, and SvelteKit (via `$env/static/public`). The `PUBLIC_` prefix is required by SvelteKit for client-side access
- SvelteKit `trailingSlash = 'always'` is set globally — all route hrefs must end with `/` (e.g. `/login/`, not `/login`), otherwise navigation breaks with the static adapter
- **Seeding:** Air (`task dev`) builds with `-tags dev`, causing `seed.Run(app)` to execute automatically at server startup using `internal/pocketbase/seed/data.go`. Edit `data.go` to change seed data.
- **Dev vs prod builds:** `air` (dev) compiles with `-tags dev`; `task build:backend` compiles without it. The `//go:build dev` constraint in `internal/pocketbase/seed/` means the seeder is a no-op in production binaries.
- **Dev DB is ephemeral:** Air compiles the server to `tmp/server.exe` and `clean_on_exit = true` wipes `tmp/` on exit — including `tmp/pb_data/` where PocketBase stores its dev database. This is intentional: each `task dev` session starts with a clean slate. TypeScript type generation (`task typegen`) therefore uses `--url` mode against the live server rather than reading the DB file directly.
- **`atlas/` directory:** Snapshots of predecessor projects (`HaloCaster`, `xemu-cartographer-legacy`) kept as porting reference. **Treat every artifact here as unverified** — offsets, patterns, and APIs must be re-confirmed against current xemu/library behavior before being copied into the live tree. Not part of the build, not imported, not modified. When in doubt, read `atlas/README.md` first. Contents are gitignored (local-only) by default.
- **CI** ([.github/workflows/ci.yml](.github/workflows/ci.yml)): three jobs gate `main`. **frontend** runs `pnpm lint`, `pnpm check`, `pnpm test`, `pnpm build`; **backend** runs `go vet ./...` and `go build` (note: **not** `go test`); **e2e** downloads the built `pb_public/` and runs Playwright. Backend unit tests are local-only — run `go test ./...` before pushing scraper/podman/proxy changes.
- **pnpm pinned to v11:** both `Containerfile` (`corepack prepare pnpm@11`) and `.github/workflows/ci.yml` (`pnpm/action-setup` `version: 11`); pnpm v11 requires Node 22+ (already the baseline). The dependency build-script allowlist lives in `sveltekit/pnpm-workspace.yaml` under v11's `allowBuilds` map (`esbuild: true`, `sqlite3: true`), which replaced v10's `onlyBuiltDependencies` list; pnpm ignores a `pnpm` block in `package.json`, and the `Containerfile`'s frontend stage must copy this file into the build context. `pnpm-lock.yaml` (`lockfileVersion: '9.0'`) is unchanged — v11 reads the existing lockfile as-is, so there's no lockfile churn — but because build-script approvals now use v11's `allowBuilds` syntax (which v10 doesn't recognize), the project targets v11.

## Containers (xemu + browser pairs)

The `internal/podman/` package shells out to the `podman` CLI to provision xemu + Firefox-kiosk container pairs. Routes live under `/api/admin/containers/*` (admin auth required). Disabled by default; opt in by setting `CONTAINERS_ENABLED=true` in `.env`.

In-container boot logic lives in [containers/xemu/init/](containers/xemu/init/) (numbered shell scripts run in order: `01-setup-toml.sh`, `02-patch-toml.sh`, `03-setup-hdd.sh`). QMP sockets are bind-mounted into [containers/xemu/qmp/](containers/xemu/qmp/), which is what the discovery watcher polls.

### Prerequisites

- **Rooted Podman + crun.** `/dev/kvm` + `/dev/dri` device passthrough and `NET_ADMIN`/`NET_RAW` caps don't work rootless. On CachyOS: `sudo pacman -S podman crun`, then `sudo systemctl enable --now podman.socket`. The Go binary itself doesn't need to run as root — `podman` does (sudo or rootful service). The `crun` runtime is non-optional: with the default `runc` (1.4.x) on some hosts, the jlesage/firefox kiosk's Xvnc rejects all X clients ("Authorization required, but no authorization protocol specified") and the noVNC view stays black. [.env.example](.env.example) defaults `CONTAINERS_PODMAN_CMD=sudo -n podman --runtime=crun` to select it.
- **Pre-pull images** (auto-pulls on first start, but pre-pulling avoids surprises):

  ```sh
  sudo podman pull lscr.io/linuxserver/xemu:latest
  sudo podman pull docker.io/jlesage/firefox
  ```

- **`_default.qcow2` baseline.** [containers/xemu/init/03-setup-hdd.sh](containers/xemu/init/03-setup-hdd.sh) copies from `$HDD_DIR/_default.qcow2` (`DEFAULT_HDD_NAME` in [containers/xemu/init/.env](containers/xemu/init/.env)). Without this file, container boot fails. Drop a baseline qcow2 at `containers/xemu/shared/hdds/_default.qcow2` before the first `Start`. Easiest paths: `qemu-img create -f qcow2 ./containers/xemu/shared/hdds/_default.qcow2 8G` for a blank image, or copy a pre-configured xemu HDD.

### Endpoints

All routes inherit `RequireAuth + RequireAdmin` middleware (see [internal/pocketbase/routes/containers/](internal/pocketbase/routes/containers/)).

| Method | Path                                 | Body             | Returns                         |
| ------ | ------------------------------------ | ---------------- | ------------------------------- |
| GET    | `/api/admin/containers`              | —                | `200` array of `ContainerInfo`  |
| POST   | `/api/admin/containers`              | `{"name":"..."}` | `201` new `ContainerInfo`       |
| GET    | `/api/admin/containers/{name}`       | —                | `200 {"status":"running"\|...}` |
| POST   | `/api/admin/containers/{name}/start` | —                | `204`                           |
| POST   | `/api/admin/containers/{name}/stop`  | —                | `204`                           |
| DELETE | `/api/admin/containers/{name}`       | —                | `204`                           |
| DELETE | `/api/admin/containers/{name}/files` | —                | `204` (or `409` if running)     |
| POST   | `/api/admin/containers/cleanup`      | —                | `200 {"deleted":[…]}`           |

`DELETE /{name}/files` wipes the on-disk bind-mount sources (xemu config + Firefox profile) without touching the container record — useful for resetting a corrupted profile (e.g. stale Firefox `.parentlock`) without losing the entry. Refuses if either container half is currently `running`/`paused`/`restarting` so we don't yank the rug under a live X session.

`POST /cleanup` walks `containers/browser/config-*`, `containers/xemu/configs/*`, and `containers/xemu/shared/hdds/*` and removes any entry that doesn't correspond to a live container record. **Files in `containers/xemu/shared/hdds/` whose basename starts with `_` (e.g. `_default.qcow2`) are treated as protected baselines and skipped.** Both halves rely on `Manager.runSudo` to escalate when the entries are root-owned (which is the default — see below).

Both halves of the container pair run as **root inside the container** (`PUID=PGID=os.Getuid()`, `USER_ID=GROUP_ID=os.Getuid()`). xemu needs that for `NET_ADMIN`/`NET_RAW` (pcap netplay binds raw sockets on the host interface, and Linux projects container caps into the effective set only for root). The browser side matches for symmetry. With `task dev` typically run under `sudo` (so the scraper can read host-side process state), this means the in-container init writes back into bind mounts as host root — leaving config dirs and HDD files root-owned on the host. The two cleanup endpoints above (and the `Cleanup files` button on `/containers/`) are the operator's path to remove those without manually escalating in a shell.

Per-instance ports are allocated stride-wise from `CONTAINERS_PORT_BASE` (default 3100). For `index=0`: HTTP=3100, HTTPS=3101, WS=3102, BrowserWeb=3103, VNC=3104. The browser container points its kiosk Firefox at `https://localhost:<XemuHTTPS>` so the UI is visible at `http://localhost:<BrowserWeb>`.

### Smoke test

1. `task dev` (with `CONTAINERS_ENABLED=true` in `.env`).
2. Authenticate as a superuser (or any user with `isAdmin=true`) via the PocketBase admin UI at `http://localhost:8090/_/`, grab the JWT.
3. `curl -X POST http://localhost:8090/api/admin/containers -H "Authorization: $JWT" -d '{"name":"smoke"}'` → `201` with allocated ports.
4. `curl -X POST http://localhost:8090/api/admin/containers/smoke/start -H "Authorization: $JWT"` → `204`.
5. Browse `http://localhost:3103` → Firefox kiosk → xemu Selkies stream.
6. Server stdout shows `discovery: socket up name=smoke path=...` once the QMP socket appears in `containers/xemu/qmp/`.
7. `curl -X POST .../smoke/stop` then `curl -X DELETE .../smoke` to tear down.

### Discovery watcher

When `CONTAINERS_ENABLED=true` and `CONTAINERS_SOCKET_DIR` is set, the server starts a goroutine that polls the directory every 2s for new `.sock` files:

- `onAdd(name, sock)` → `scrMgr.Start(name, sock)` in a goroutine. Failures (xemu init error, unknown title ID from `scraper.Detect`) are logged but don't kill the watcher.
- `onRemove(name)` → `scrMgr.Stop(name)` (idempotent — silent no-op if the runner is already gone).

So once a smoke-test container's QMP socket appears in `containers/xemu/qmp/`, a scraper auto-attaches; once it disappears, the runner is torn down. Manual `/api/admin/scraper/start` is still useful for sockets outside the watched directory.
