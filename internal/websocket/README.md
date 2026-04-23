# internal/websocket

WebSocket server using [coder/websocket](https://github.com/coder/websocket).

## Responsibilities

Provides a WebSocket endpoint mounted on PocketBase's ServeMux router. Handles connection upgrades with optional JWT authentication, and provides a Hub for managing connected clients, rooms, and message routing. PocketBase hooks and Disgo handlers push messages to clients via the Hub singleton.

## Why coder/websocket

`coder/websocket` is a lightweight, stdlib-compatible WebSocket library. It has native `context.Context` support, safe concurrent writes without external mutexes, zero external dependencies, and works directly with `net/http` handlers. It is the actively maintained successor to `nhooyr/websocket`.

## Subdirectories

| Directory    | Purpose                                                                |
|--------------|------------------------------------------------------------------------|
| `handlers/`  | Self-registering message type handlers (one per file, dispatched by Hub) |
| `rooms/`     | Room type definitions with guard lists (one per file, self-registering)  |
| `resolvers/` | WS state lookups via `*guards.Services` — one function per file          |
| `actions/`   | Reusable WS operations — one exported function per file                  |

## Key Files

- `hub.go` — `Hub` struct, `NewHub()`, `Run()`, `Stop()`, routing, singleton (`SetInstance()`/`Instance()`), `SetServices()` for cross-system access, `*Raw` methods (`BroadcastRaw`, `SendToUserRaw`, `SendToRoomRaw`) that satisfy `wsiface.Service`
- `client.go` — `Client` struct, `readPump()`, `writePump()`, `UserID()`
- `handler.go` — `NewHandler(hub, app)` returns PocketBase-compatible route handler
- `message.go` — `Message` struct + type constants (`TypeBroadcast`, `TypeRoom`, `TypeDirect`, `TypeJoinRoom`, `TypeLeaveRoom`, `TypeError`)
- `handlers/allhandlers.go` — `Event` type (carries `Services` for cross-system access) + `HandlerFunc` + registry (`register()` / `Get()`)
- `handlers/guards.go` — `RequireAuth()`, `RequireRole()`, `RequireAdmin()` guard functions

## Auth Flow

1. Browser connects: `new WebSocket("ws://host/api/ws?token=PB_JWT")`
2. Handler checks for `?token=` query parameter
3. If present, validates with `app.FindAuthRecordByToken(token, core.TokenTypeAuth)`
4. Valid token → Client is tagged with the authenticated user record
5. Invalid or missing token → connection stays open as anonymous
6. Connection is upgraded with `websocket.Accept()`
7. Client is registered with the Hub

## Message Routing

```
Browser → readPump → Hub dispatches by message.Type
├── Registered handler found → handler(Event)
│   ├── e.Broadcast()  → all connected clients
│   ├── e.SendToRoom() → clients in a specific room
│   ├── e.SendToUser() → specific user's connections
│   ├── e.JoinRoom()   → add sender to room
│   └── e.LeaveRoom()  → remove sender from room
└── No handler → default broadcast to all clients
```

PocketBase hooks and Disgo event handlers call Hub methods directly via the singleton or through the `Services` interface:

```go
// Via singleton (internal use, takes Message struct):
ws.Instance().Broadcast(ws.Message{Type: "new_post", Payload: payload})
ws.Instance().SendToUser(userID, ws.Message{Type: "notification", Payload: payload})
ws.Instance().SendToRoom("lobby", ws.Message{Type: "chat", Payload: payload})

// Via Services interface (cross-system use, takes []byte):
svc.WS.BroadcastRaw(jsonBytes)
svc.WS.SendToUserRaw(userID, jsonBytes)
svc.WS.SendToRoomRaw("lobby", jsonBytes)
```

## Guards

Guard functions in `handlers/guards.go` enforce access control at the top of message handlers. They mirror the Disgo guards pattern (`internal/disgo/guards/`) — explicit checks that return errors on failure.

| Guard | Checks |
|-------|--------|
| `RequireAuth(e)` | `e.User != nil` — client is authenticated |
| `RequireRole(e, role)` | Authenticated + `e.User.GetString("role") == role` |
| `RequireAdmin(e)` | Authenticated + `e.User.GetBool("isAdmin")` |

Handlers use `e.SendError(code, message)` to notify the client when a guard fails.

### Admin-only room

```go
func handleJoinRoom(e *Event) {
    if e.Room == "admin-chat" {
        if err := RequireAdmin(e); err != nil {
            e.SendError("forbidden", "admin access required")
            return
        }
    }
    e.JoinRoom(e.Room)
}
```

### Public room, authenticated posting

```go
func handleChat(e *Event) {
    if err := RequireAuth(e); err != nil {
        e.SendError("unauthorized", "must be logged in to send messages")
        return
    }
    e.SendToRoom(e.Room, e.Payload)
}
```

Anonymous users still receive messages (they're in the room), they just can't trigger the chat handler.

## Origin Policy

Set `WS_ALLOWED_ORIGINS` (comma-separated) to restrict WebSocket origins in production. If unset, all origins are accepted for development convenience.

```sh
# Production
WS_ALLOWED_ORIGINS=yourdomain.com,*.yourdomain.com

# Development (default — no env var needed)
# All origins accepted
```

## Adding New Items

### Message handler (self-registering)

1. Copy `handlers/handler.go.example`, rename to your message type (e.g., `chat.go`)
2. Add an `init()` function that calls `register("your_type", handlerFunc)`
3. Done — Hub dispatches messages with `"type":"your_type"` to your handler automatically

### Action (no registry)

1. Create a new file in `actions/` named after the operation (e.g., `broadcast_new_post.go`)
2. Export a single function that calls `ws.Instance()` to get the Hub
3. Call it from any trigger: PocketBase hooks, Disgo commands, or custom routes
