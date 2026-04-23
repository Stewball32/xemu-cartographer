# cmd/server

Go entry point for the application. Contains `main.go`.

## Responsibilities

This is where all components are wired together before `app.Start()` is called:

1. Create the PocketBase app instance
2. Register PocketBase hooks (`internal/pocketbase/hooks`)
3. In OnServe:
   - Register collection schemas, OAuth providers, and custom API routes
   - Initialize WebSocket Hub, start its `Run()` goroutine, mount `/api/ws` endpoint
   - Start the Disgo Discord bot (non-blocking)
   - Build the `guards.Services` struct connecting all three systems via interfaces
   - Inject `Services` into Hub, hooks, bot, and commands via `SetServices()`
4. Register OnTerminate hook for cleanup
5. Call `app.Start()` — blocking, runs PocketBase's HTTP server

## Expected Files

- `main.go` — wires and starts the application
- `config.go` (optional) — loads environment variables / config struct
