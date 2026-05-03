# xemu-cartographer

> **For AI assistants:** See [CLAUDE.md](CLAUDE.md) for development commands, conventions, and implementation details, and [ROADMAP.md](ROADMAP.md) for the migration plan.

Real-time game-state scraper for Xbox titles running in [xemu](https://xemu.app/). Orchestrates containerized xemu+browser pairs, reads memory via QMP + `/proc/<pid>/mem`, broadcasts live state over WebSocket, and persists match records to PocketBase. Halo: CE support comes first, Halo 2 follows.

Built on a prior Go+SvelteKit implementation preserved at [atlas/xemu-cartographer-legacy/](atlas/xemu-cartographer-legacy/), with HaloCaster's Python/C# memory work at [atlas/HaloCaster/](atlas/HaloCaster/) as the richest source of Halo: CE offsets. Everything under `atlas/` is reference-only and must be re-verified before porting into the live tree.

## Tech Stack

| Layer               | Technology                                                                           |
| ------------------- | ------------------------------------------------------------------------------------ |
| Backend / Auth / DB | [PocketBase](https://pocketbase.io/) v0.36.7 (Go framework, ServeMux router)         |
| Discord Bot         | [Disgo](https://github.com/disgoorg/disgo) v0.19.3                                   |
| WebSocket Server    | [coder/websocket](https://github.com/coder/websocket)                                |
| Frontend            | [SvelteKit](https://kit.svelte.dev/) 2 + [Skeleton UI v4](https://www.skeleton.dev/) |
| Frontend Build      | `@sveltejs/adapter-static` → served by PocketBase                                    |
| Build Orchestration | [Taskfile](https://taskfile.dev/)                                                    |
| Container           | [Podman](https://podman.io/)                                                         |

## Architecture Overview

```
┌──────────────────────────────────────────────┐
│            Go Binary (cmd/server)            │
│                                              │
│  ┌──────────────┐   ┌─────────────────────┐  │
│  │  PocketBase  │   │      Disgo Bot      │  │
│  │  - REST API  │   │  - Slash Commands   │  │
│  │  - Auth/JWT  │   │  - Event Listeners  │  │
│  │  - SQLite    │   └─────────────────────┘  │
│  │  - ServeMux  │                            │
│  │    Router    │   ┌─────────────────────┐  │
│  └──────┬───────┘   │      WebSocket      │  │
│         │           │  (coder/websocket)  │  │
│         │           │  - Optional JWT     │  │
│         │           │  - Hub / Rooms      │  │
│         │           └─────────────────────┘  │
│         │                                    │
│  guards/Services → cross-system DI           │
│  PB Hooks → Discord notifications            │
│  PB Hooks → WS Hub broadcasts                │
│  PB Routes → Auth-gated page serving         │
│                                              │
└─────────┬────────────────────────────────────┘
          │ serves
┌─────────▼───────┐
│   pb_public/    │ ← SvelteKit static build
│   (SvelteKit)   │
└─────────────────┘
```

## Project Structure

```
.
├── cmd/server/                # Go entrypoint
│   └── main.go
├── internal/
│   ├── guards/                # Unified cross-system guards + Services DI
│   │   ├── interfaces/
│   │   │   ├── discord/       # Per-method Discord interfaces (one per file)
│   │   │   ├── websocket/     # Per-method WS interfaces (one per file)
│   │   │   └── pocketbase/    # Per-method PB interfaces (one per file)
│   │   ├── services.go        # Services struct (bundles all system interfaces)
│   │   ├── guard.go           # GuardFunc type definition
│   │   └── require_*.go       # Guard implementations
│   ├── pocketbase/
│   │   ├── service.go         # PB service wrapper (implements pbiface.Service)
│   │   ├── schema/            # Programmatic collection definitions
│   │   ├── hooks/             # Record event hooks (PB → Discord, PB → WS)
│   │   ├── routes/            # Custom API routes + protected page serving
│   │   │   └── middleware/    # Auth middleware, role checks
│   │   ├── oauth/             # OAuth2 provider configuration
│   │   ├── actions/           # Reusable PB data operations
│   │   └── resolvers/         # PB data lookups (one function per file)
│   ├── disgo/
│   │   ├── bot.go             # Bot client + interface methods + lifecycle
│   │   ├── commands/          # Slash command definitions and handlers
│   │   ├── events/            # Discord gateway event listeners
│   │   ├── actions/           # Reusable Discord API calls
│   │   ├── resolvers/         # Discord data lookups via Services
│   │   ├── components/        # UI builders (buttons, embeds, rows)
│   │   └── guards/            # Bot-side permission checks
│   └── websocket/
│       ├── hub.go             # Client registry, rooms, message routing
│       ├── handler.go         # WS upgrade with optional JWT auth
│       ├── client.go          # Single connection read/write pumps
│       ├── message.go         # Wire format for WS messages
│       ├── handlers/          # Self-registering message type handlers
│       ├── rooms/             # Room type definitions with guard lists
│       └── resolvers/         # WS state lookups via Services
├── sveltekit/                 # SvelteKit frontend (Skeleton UI v4, adapter-static → pb_public/)
├── .env.example               # Env template (shared by backend + frontend via envDir)
├── .air.toml                  # Go hot reload config
├── .gitignore
├── Taskfile.yml               # Build orchestration
├── Containerfile              # Multi-stage Podman/Docker build
├── compose.yml                # Container compose config
├── go.mod
└── LICENSE
```

## Prerequisites

- **Go 1.25+** — runs the backend; [go.dev/dl](https://go.dev/dl)
- **pnpm** _(preferred)_ — package manager for the frontend; `npm install -g pnpm`. npm and yarn work but the project is developed with pnpm.
- **Podman** _(optional)_ — for building and running containers; Docker works as a drop-in alternative.
- **Task** _(optional)_ — task runner for dev commands like `task dev` and `task build`; `sudo env GOBIN=/usr/local/bin go install github.com/go-task/task/v3/cmd/task@latest`. Without it, run `air` and `pnpm dev` in separate terminals.
- **Air** _(optional)_ — Go hot-reload; auto-rebuilds the server on `.go` file saves; `sudo env GOBIN=/usr/local/bin go install github.com/air-verse/air@latest`. Without it, use `go run ./cmd/server serve` and restart manually.

  > **Why `/usr/local/bin`?** Installing system-wide (rather than the default `~/go/bin`) puts both binaries on root's PATH, so `sudo task dev` works without `command not found` errors — the same applies to `air`, which `task` shells out to. If you never plan to run with sudo, drop the `sudo env GOBIN=/usr/local/bin` prefix and they'll install to `~/go/bin` as usual; or symlink an existing `~/go/bin/{task,air}` into `/usr/local/bin` if you'd rather keep the user-level install and just expose it to root.

## Quick Start

1. **Clone:**

   ```bash
   git clone https://github.com/Stewball32/xemu-cartographer.git
   cd xemu-cartographer
   ```

2. **Configure environment:**

   ```bash
   cp .env.example .env
   # Only PUBLIC_PB_PORT is required. Discord bot and OAuth vars are optional.
   ```

3. **Install frontend dependencies:**

   ```bash
   cd sveltekit && pnpm install && cd ..
   ```

4. **Run in development:**

   ```bash
   task dev
   ```

5. **Build for production:**
   ```bash
   task build
   ./bin/server serve
   ```

## Containers

```bash

# Build image

task container:build

# Run (PUBLIC_PB_PORT defaults to 8090, set in .env)

task container:run
```

For multiple instances on the same machine, set a unique `PUBLIC_PB_PORT` in each project's `.env`. Route traffic with cloudflared or a reverse proxy.

## Notes

- `pb_data/` — PocketBase runtime data (SQLite DB, uploads). Created at runtime, gitignored. Nothing wipes it automatically — if data disappears, check for `git clean -fdx` in your workflow.
- `pb_public/` — SvelteKit build output. Created by `task build:frontend`, gitignored.
- Schema can be managed via PocketBase admin UI or programmatically in `internal/pocketbase/schema/`.
- Protected pages are served through auth-gated custom routes; public pages are served directly from `pb_public/`.
