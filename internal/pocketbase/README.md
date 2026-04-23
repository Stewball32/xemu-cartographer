# internal/pocketbase

PocketBase application setup and customization.

## Key Files

- `service.go` — `Service` struct wrapping `core.App`, implements `pbiface.Service` by delegating to resolvers. Created in `main.go` via `pb.NewService(app)` and injected into the `guards.Services` struct.

## Subdirectories

| Directory          | Entry point     | Purpose                                             |
|--------------------|-----------------|-----------------------------------------------------|
| `schema/`          | `RegisterAll()` | Programmatic collection definitions                 |
| `hooks/`           | `RegisterAll()` | Record lifecycle hooks (create, update, delete)     |
| `routes/`          | `RegisterAll()` | Custom API routes on PocketBase's ServeMux          |
| `routes/middleware/`| `Init()`       | Per-route middleware functions + global middleware registry |
| `routes/admin/`    | `RegisterAll()` | Route group for `/api/admin` (auth + admin middleware) |
| `oauth/`           | `RegisterAll()` | OAuth2 provider configuration for auth collections |
| `actions/`         | (none)          | Reusable PB data operations — one exported function per file |
| `resolvers/`       | (none)          | PB data lookups — one exported function per file    |

Each subdir has a `.go.example` file showing the pattern for adding new domains.

## Wiring (cmd/server/main.go)

```go
hooks.RegisterAll(app)       // callback registration, fires later

app.OnServe().BindFunc(func(se *core.ServeEvent) error {
    schema.RegisterAll(app)  // needs running DB → error
    oauth.RegisterAll(app)   // needs users collection from schema → error
    routes.RegisterAll(se)   // middleware.Init → groups → ungrouped routes

    // ... WebSocket Hub + Disgo Bot setup ...

    pbSvc := pb.NewService(app)
    svc := &guards.Services{App: app, WS: hub, PB: pbSvc, Discord: bot}
    hub.SetServices(svc)
    hooks.SetServices(svc)   // cross-system access for hooks
    commands.SetServices(svc) // cross-system access for Discord commands
    return se.Next()
})
```

Hooks can use the `svc` package-level var in `routine.FireAndForget` goroutines for cross-system calls (e.g., `svc.Discord.SendNotification()`, `svc.WS.BroadcastRaw()`).

## Route groups

Route groups share a URL prefix and middleware. Each group is a subfolder of `routes/` (its own Go package).

```
routes/
  allroutes.go          ← orchestrator (never changes for new groups)
  allgroups.go          ← one line per group (edit this to add a group)
  hello.go              ← ungrouped route
  admin/
    allroutes.go        ← group definition + route registry
    stats.go            ← route (inherits group middleware)
```

**Execution order in `RegisterAll`:**
1. `middleware.Init(se)` — global middleware
2. `registerAllGroups(se)` — creates groups + registers their routes
3. Ungrouped routes

Convention: `allroutes.go` is the wiring file at every level. Everything else is a route.

Groups are hierarchical — a route belongs to exactly one group and inherits all of its middleware. To combine middleware from different concerns on a single route, chain per-route `.Bind()`/`.BindFunc()` calls.

## Adding a new domain

### Schema, hooks, ungrouped routes (self-registering)

1. Copy the `.go.example` file in the relevant subdir, rename to your domain (e.g., `comments.go`)
2. Add an `init()` function that calls `register()` with your registration function
3. Done — no other files need editing

### Route group (folder per group)

1. Create a new folder under `routes/` (e.g., `routes/editor/`)
2. Add `allroutes.go` — define the group var, prefix, middleware, registry, and `RegisterAll()`
3. Add a `routes.go.example` template for the group
4. Import the group package in `routes/allgroups.go` and call its `RegisterAll`
5. Add route files to the folder — each self-registers via `init()` + `register()`

### Route in an existing group (self-registering)

1. Copy the `routes.go.example` in the group folder, rename to your route (e.g., `users.go`)
2. Add an `init()` function that calls `register()` — routes use `Group.GET(...)` etc.
3. Done — no other files need editing

### OAuth (self-registering)

1. Copy `oauth.go.example`, rename to your provider (e.g., `github.go`)
2. Add an `init()` function that calls `register()` with your provider factory
3. Set the corresponding env vars (`PROVIDER_CLIENT_ID`, `PROVIDER_CLIENT_SECRET`)
4. Done — no other files need editing

### Action (no registry)

Actions have no `init()` or `RegisterAll()` — they're just exported functions. Each file exports one function taking `app *pocketbase.PocketBase` as the first parameter.

1. Create a new file in `actions/` named after the operation (e.g., `find_user_by_discord_id.go`)
2. Export a single function (e.g., `FindUserByDiscordID(app, discordID)`)
3. Call it from any trigger: routes, hooks, or Discord commands via cross-package import

Actions are the shared logic layer — when multiple triggers (a route, a hook, and a Discord command) all need the same PB operation, extract it here.

### Per-route middleware (one function per file)

Middleware has no central registration step. Each function is attached directly to routes where needed — PocketBase calls this "registering" a middleware, but it just means calling `.Bind()` or `.BindFunc()` on a route or group.

1. Create a new file in `routes/middleware/` named after your middleware (e.g., `ratelimit.go`)
2. Export a single middleware function
3. Attach it to routes via `.Bind()` or `.BindFunc()` (see reference table below)

### Global middleware (self-registering)

1. Copy `routes/middleware/global.go.example`, rename (e.g., `cors.go`)
2. Add an `init()` function that calls `registerGlobal()` with `se.Router.BindFunc(...)`
3. Done — runs on every request before groups and routes

## Middleware reference

Use `.Bind()` for `*hook.Handler` types (e.g., `apis.RequireAuth()`), `.BindFunc()` for plain `func(*core.RequestEvent) error` closures. Chain multiple middleware left-to-right on a single route.

| Function                     | File              | Binding     | Purpose                                |
|------------------------------|-------------------|-------------|----------------------------------------|
| `apis.RequireAuth()`         | (built-in)        | `.Bind()`   | Rejects unauthenticated requests       |
| `apis.RequireGuestOnly()`    | (built-in)        | `.Bind()`   | Rejects authenticated requests         |
| `apis.RequireSuperuserAuth()`| (built-in)        | `.Bind()`   | Restricts to superusers                |
| `middleware.RequireAuth()`   | `auth.go`         | `.Bind()`   | Wrapper around apis.RequireAuth()      |
| `middleware.RequireCustomAuth()` | `custom_auth.go` | `.BindFunc()` | Manual `e.Auth` nil check        |
| `middleware.RequireRole(r)`  | `role.go`         | `.BindFunc()` | Checks user's "role" field           |
| `middleware.RequireAdmin()`  | `admin.go`        | `.BindFunc()` | Checks user's "isAdmin" field        |
