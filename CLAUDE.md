# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

> See also: [README.md](README.md) for project overview, tech stack, architecture diagram, and quick-start guide.

## Reference docs

Before writing or reviewing code that touches a third-party library where the API may have drifted from your training data, consult up-to-date docs rather than guessing.

- **Skeleton UI v4** — [sveltekit/docs/skeleton-llms.txt](sveltekit/docs/skeleton-llms.txt) is a table of contents of Skeleton's official docs (components, theming, Tailwind v4 integration). Read it first to locate the right page, then WebFetch the specific page under `https://www.skeleton.dev/` (e.g. `https://www.skeleton.dev/docs/svelte/framework-components/app-bar.md`, `https://www.skeleton.dev/docs/svelte/tailwind-components/buttons`). Always use the **Svelte** section, not React.
- **SvelteKit, PocketBase JS SDK, Disgo, Tailwind v4** — WebFetch the official docs site (`kit.svelte.dev`, `pocketbase.io/docs`, `disgo.dev`, `tailwindcss.com`) rather than inventing an API.

## Development Commands

```sh

# Install task runner and hot reload

go install github.com/go-task/task/v3/cmd/task@latest
go install github.com/air-verse/air@latest

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

Single Go binary (`cmd/server`) runs three concurrent systems:

1. **PocketBase** — REST API, auth (JWT), SQLite database, static file server (serves `pb_public/`), uses `net/http.ServeMux` router
2. **Disgo Discord bot** — connects via gateway in PocketBase's OnServe hook, non-blocking
3. **WebSocket handler** (`coder/websocket`) — custom route on PocketBase's router with optional JWT auth, Hub for managing clients/rooms/broadcasting

The SvelteKit frontend is built with `@sveltejs/adapter-static` into `pb_public/`, which PocketBase serves automatically. The `fallback: 'index.html'` config enables SPA-style client-side routing.

Protected pages can be served through custom PocketBase routes that validate JWT auth before serving the static file, while public pages are served directly from `pb_public/`.

## Backend Structure

### Startup sequence (`cmd/server/main.go`)

1. Create PocketBase app instance
2. Register record lifecycle hooks — `hooks.RegisterAll(app)` (callback registration, fires later)
3. In OnServe hook:
   - Register collection schemas (`schema.RegisterAll(app)`)
   - Register OAuth2 providers (`oauth.RegisterAll(app)`) — must run after schema
   - Register custom API routes (`routes.RegisterAll(se)`)
   - Initialize WebSocket Hub, start its Run() goroutine, mount `/api/ws` endpoint
   - Start Disgo bot gateway connection (non-blocking)
   - Wire cross-system `Services` struct — connects all three systems via interfaces
4. Register OnTerminate hook to shut down Disgo bot cleanly
5. Call `app.Start()` (blocking)

### Key packages

| Package                                 | Purpose                                                             |
| --------------------------------------- | ------------------------------------------------------------------- |
| `internal/guards`                       | Unified cross-system guards, `Services` struct, `GuardFunc` type    |
| `internal/guards/interfaces/discord`    | Per-method Discord interfaces (Membership, Roles, Notify, Voice)    |
| `internal/guards/interfaces/websocket`  | Per-method WS interfaces (Connected, Rooms, Broadcast)              |
| `internal/guards/interfaces/pocketbase` | Per-method PB interfaces (Users)                                    |
| `internal/pocketbase`                   | PB service wrapper — implements `pbiface.Service`                   |
| `internal/pocketbase/schema`            | Programmatic collection definitions — one file per domain           |
| `internal/pocketbase/hooks`             | Record event hooks — fire Discord notifications, push to WS clients |
| `internal/pocketbase/oauth`             | OAuth2 provider config — env-driven, self-registering, one per file |
| `internal/pocketbase/routes`            | Custom endpoints + protected page serving via auth-gated routes     |
| `internal/pocketbase/routes/middleware` | Auth middleware, role checks, global middleware registry            |
| `internal/pocketbase/routes/admin`      | Route group for `/api/admin` — auth + admin middleware              |
| `internal/pocketbase/seed`              | In-process dev seeder — `seed.go` (`//go:build dev`) + `stub.go` (`//go:build !dev`) + `data.go` defines seed vars |
| `internal/pocketbase/resolvers`         | PB data lookups — one exported function per file                    |
| `internal/websocket`                    | WebSocket Hub, client management, message routing, JWT auth upgrade |
| `internal/websocket/handlers`           | Self-registering WS message handlers — one per file                 |
| `internal/websocket/rooms`              | Room type definitions with guard lists — one per file               |
| `internal/websocket/resolvers`          | WS state lookups via Services — one exported function per file      |
| `internal/websocket/actions`            | Reserved for WS outbound action helpers (currently only `.go.example` stub) |
| `internal/disgo`                        | Bot client setup: NewBot(), OpenGateway(), Close(), action methods  |
| `internal/disgo/commands`               | Slash command definitions and handler functions                     |
| `internal/disgo/events`                 | Discord gateway event listeners for non-interaction events          |
| `internal/disgo/actions`                | Reusable Discord API calls — one exported function per file         |
| `internal/disgo/resolvers`              | Discord data lookups via Services — one exported function per file  |
| `internal/disgo/components`             | UI builder factories (buttons, embeds, rows, selects, modals)       |

## Frontend Structure

- **UI framework:** Skeleton UI v4 (Svelte 5 + Tailwind CSS v4), cerberus theme
- **API client:** PocketBase JS SDK (`pocketbase` npm package) — singleton in `src/lib/pocketbase.ts`; in dev points to `http://localhost:PORT`, in production passes `undefined` (same-origin relative)
- **Auth store:** `src/lib/stores/auth.svelte.ts` — uses Svelte 5 runes (`$state`/`$derived`), not writable stores
- **Mode store:** `src/lib/stores/mode.svelte.ts` — dark/light mode toggle, persisted in `localStorage`; call `mode.toggle()` or `mode.set('dark'|'light')`
- **Toaster:** `src/lib/stores/toaster.ts` — global Skeleton toast singleton (`toaster`); import and call `toaster.trigger(...)` from any component
- **Navigation:** `src/lib/config/navigation.ts` — central nav link config consumed by all four layout nav components; edit here to add/remove nav links
- **App config:** `src/lib/config/app.ts` — exports `APP_NAME` (displayed app name) and `OAUTH_PROVIDERS` (display labels + icons per provider); actual enabled providers are discovered at runtime from PocketBase's `listAuthMethods()` API
- **WebSocket:** Browser native `WebSocket` API connecting to `/api/ws?token=PB_JWT`
- **Routing:** SvelteKit file-based routing in `sveltekit/src/routes/`; `+layout.ts` sets `ssr = false`, `prerender = true`, `trailingSlash = 'always'` globally
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

The three main systems (PocketBase, Disgo, WebSocket) never import each other. Cross-system communication is mediated through:

1. **Interfaces** (`internal/guards/interfaces/`) — one interface per file, organized in per-system subdirectories (`discord/`, `websocket/`, `pocketbase/`). Small interfaces compose into aggregate `Service` interfaces via embedding.
2. **Services struct** (`internal/guards/services.go`) — bundles all system references. Fields are nil if the system is not running.
3. **Dependency injection** — `main.go` builds the `Services` struct and injects it into all three systems via `SetServices()`.

Handler flow: **Trigger → Resolve → Guard → Action**

- **Resolvers** stay in their own package (`pocketbase/resolvers/`, `disgo/resolvers/`, `websocket/resolvers/`) and only talk to their own system
- **Guards** (`internal/guards/`) take `*Services` and check cross-system permissions
- **Actions** are called through `Services` interfaces (e.g., `svc.Discord.SendNotification()`, `svc.WS.BroadcastRaw()`)

Handlers orchestrate by calling resolvers/guards/actions from multiple systems — no resolver or guard calls sideways into another package's resolvers.

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
- Interface files use one-interface-per-file convention for merge-safe parallel development
- Custom routes registered in OnServe take priority over pb_public/ static file serving
- `PUBLIC_PB_PORT` in root `.env` — single port variable used by Taskfile, compose, Containerfile, and SvelteKit (via `$env/static/public`). The `PUBLIC_` prefix is required by SvelteKit for client-side access
- SvelteKit `trailingSlash = 'always'` is set globally — all route hrefs must end with `/` (e.g. `/login/`, not `/login`), otherwise navigation breaks with the static adapter
- **Seeding:** Air (`task dev`) builds with `-tags dev`, causing `seed.Run(app)` to execute automatically at server startup using `internal/pocketbase/seed/data.go`. Edit `data.go` to change seed data.
- **Dev vs prod builds:** `air` (dev) compiles with `-tags dev`; `task build:backend` compiles without it. The `//go:build dev` constraint in `internal/pocketbase/seed/` means the seeder is a no-op in production binaries.
- **Dev DB is ephemeral:** Air compiles the server to `tmp/server.exe` and `clean_on_exit = true` wipes `tmp/` on exit — including `tmp/pb_data/` where PocketBase stores its dev database. This is intentional: each `task dev` session starts with a clean slate. TypeScript type generation (`task typegen`) therefore uses `--url` mode against the live server rather than reading the DB file directly.
- **`atlas/` directory:** Snapshots of predecessor projects (`HaloCaster`, `xemu-cartographer-legacy`) kept as porting reference. **Treat every artifact here as unverified** — offsets, patterns, and APIs must be re-confirmed against current xemu/library behavior before being copied into the live tree. Not part of the build, not imported, not modified. When in doubt, read `atlas/README.md` first. Contents are gitignored (local-only) by default.

## Containers (xemu + browser pairs)

The `internal/podman/` package shells out to the `podman` CLI to provision xemu + Firefox-kiosk container pairs. Routes live under `/api/admin/containers/*` (admin auth required). Disabled by default; opt in by setting `CONTAINERS_ENABLED=true` in `.env`.

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

| Method | Path                                       | Body              | Returns                          |
| ------ | ------------------------------------------ | ----------------- | -------------------------------- |
| GET    | `/api/admin/containers`                    | —                 | `200` array of `ContainerInfo`   |
| POST   | `/api/admin/containers`                    | `{"name":"..."}`  | `201` new `ContainerInfo`        |
| GET    | `/api/admin/containers/{name}`             | —                 | `200 {"status":"running"\|...}`  |
| POST   | `/api/admin/containers/{name}/start`       | —                 | `204`                            |
| POST   | `/api/admin/containers/{name}/stop`        | —                 | `204`                            |
| DELETE | `/api/admin/containers/{name}`             | —                 | `204`                            |

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

When `CONTAINERS_ENABLED=true` and `CONTAINERS_SOCKET_DIR` is set, the server starts a goroutine that polls the directory every 2s for new `.sock` files. Currently the `onAdd`/`onRemove` callbacks just log — the wire-up to auto-start a memory scraper happens once M1+M2 land (see [ROADMAP.md](ROADMAP.md)).
