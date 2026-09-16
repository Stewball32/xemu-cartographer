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
| `rooms/`     | Room type definitions + room-name helpers (one per file, self-registering; admission is `authz.Can(room.join)`) |
| `resolvers/` | WS state lookups via `*guards.Services` — one function per file          |
| `actions/`   | Reusable WS operations — one exported function per file                  |

## Key Files

- `hub.go` — `Hub` struct, `NewHub()`, `Run()`, `Stop()`, routing, singleton (`SetInstance()`/`Instance()`), `SetServices()` for cross-system access, `*Raw` methods (`BroadcastRaw`, `SendToUserRaw`, `SendToRoomRaw`) that satisfy `wsiface.Service`; `recheckRooms()` (the re-resolve room re-check, announces each loss with `room_left`), `rebindConsole()` (the join-time console re-bind)
- `client.go` — `Client` struct (`newClient()`), `readPump()`, `writePump()`, `Principal()` (the connection's `authz.Principal`, swapped on re-resolve), `UserID()`
- `handler.go` — `NewHandler(hub, app, hooks...)` returns PocketBase-compatible route handler; resolves the principal at connect and runs the per-connection re-resolve loop
- `message.go` — `Message` struct + type constants (`TypeBroadcast`, `TypeRoom`, `TypeDirect`, `TypeJoinRoom`, `TypeLeaveRoom`, `TypeError`, `TypeRoomLeft`)
- `handlers/allhandlers.go` — `Event` type (carries `Services`, `Authz` + `Principal` for decisions) + `HandlerFunc` + registry (`register()` / `Get()`)

## Auth Flow

1. Browser connects: `new WebSocket("ws://host/api/ws?token=PB_JWT")` — or `?token=<opaque machine key>`, `?spectator=<opaque spectator key>`, `?console=<xbox console name>` (the tokenless console door)
2. `pb.ResolveWS(app, pb.Default(), r)` turns the query string into an `authz.Principal` (`pb_user`, `superuser`, `machine`, `spectator`, `device`, `anonymous`); a bad or banned credential is logged and the socket proceeds as `authz.Nobody()`
3. Connection is upgraded with `websocket.Accept()` and the `Client` carries the principal
4. Client is registered with the Hub; connect hooks (the scraper `hello`) fire with the principal so they can narrow what they announce
5. Every `authz.ReResolveInterval` (60 s) the connection re-derives its principal with `pb.ReResolve`: a credential that no longer resolves (revoked / expired key, banned / deleted user) gets `error{code:"session_revoked"}` and close code `4401`; one that still resolves is swapped in and every room it holds (`host:*` and `admin` alike) is re-decided with `room.join`, leaving the ones it can no longer enter — each loss is announced with `{"type":"room_left","room":"<room>","payload":{"reason":"forbidden"}}`, queued before the membership is dropped, and the socket stays open (the client re-joins with backoff)
6. An `anonymous` connection that named a console (`?console=`, kept in `Extra["console"]`) but is not bound to an instance is re-bound synchronously on every message that reaches a handler (`join_room` / `leave_room`, the only types the whitelist admits for it — `Hub.rebindConsole`, the same `InstanceByConsole` + `AnonymousScopes` binding the tick makes) before the handler decides — so an overlay that connected before its container came up, or whose container restarted, is admitted by its next `join_room` rather than waiting for the tick. A bound console is left to the tick, which unbinds it (with `room_left`) when the instance goes away

## Client lifecycle

Each connection has a `send` queue (`writePump` drains it) and a `done` channel. `send` is **never closed**: senders enqueue from many goroutines (Run, the scraper runners through `SendToRoomRaw`, the re-resolve loop) and a close racing any of them would be a fatal "send on closed channel". Removal (`removeClient`, on the Run goroutine only) drops the client from every index and closes `done` once (`markClosed`); `trySend` skips a closed client and keeps the non-blocking drop for a full buffer (which schedules the client's removal via `requestUnregister`, itself safe after `Stop`), `writePump` exits on `done`, `dispatch` drops a message whose sender has been removed, and `addToRoom` refuses a removed client so a queued join cannot resurrect it in the room index.

## Message Routing

```
Browser → readPump → Hub dispatches by message.Type
├── sender already unregistered → dropped
├── authz.WSSendAllowed(principal.Kind, type) false → error{code:"forbidden"} (still connected)
├── Registered handler found → handler(Event)
│   ├── e.Broadcast()  → all connected clients
│   ├── e.SendToRoom() → clients in a specific room
│   ├── e.SendToUser() → specific user's connections
│   ├── e.JoinRoom()   → add sender to room
│   └── e.LeaveRoom()  → remove sender from room
└── No handler → error{code:"unknown_type"} — never broadcast
```

The per-kind send whitelist (`authz.WSSendAllowed`): `pb_user` / `superuser` / `machine` may send any type; `spectator` / `device` only `join_room`, `leave_room`, `request_state`, `request_events`; `anonymous` only `join_room` / `leave_room`.

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

## Authorization

Admission is a single `authz.Can` call in the handler, decided against `e.Authz` (the process-wide `pb.Default()` adapter, read at dispatch) with `e.Principal` (the connection's principal). Room types in `rooms/` keep `Guards` nil — nothing walks a guard list any more; the rule table in `internal/authz` is the whole answer.

| Handler | Decision |
|---------|----------|
| `join_room` | `rooms.Resolve` (unknown type → `not_found`) → `authz.ParseRoom` (malformed host room → `bad_room`) → `Can(room.join, RoomRes(room))` (→ `forbidden`) → join + replay |
| `request_state` | per joined room: `Can(scraper.state, Instance(name))`; an aggregate feed (`host:all` / `host:summary`) `Can(room.join, <that room>)` — the same decision that admitted the join, never the other spelling |
| `request_events` | per joined instance: `Can(scraper.events, Instance(name))` — denied instances are skipped silently |
| `request_probe` | per instance (explicit or from joined rooms): `Can(scraper.probe, Instance(name))` — scope only |

A nil adapter (before `main.go` installs it) denies everything, superusers included. Handlers use `e.SendError(code, message)` to tell the client why; the error codes a client can see are `forbidden`, `not_found`, `bad_room`, `unknown_type` and `session_revoked`. The frame is `{"type":"error","room":…,"payload":{"code":…,"message":…}}`: `room` echoes the room a refused `join_room` / `leave_room` named (so a client with several joins in flight knows which one failed) and is omitted on every other error (whitelist, unknown type, eviction).

### Room shapes (`authz.ParseRoom`)

```
host:<instance>          bare per-instance room — pb_user only (rostered gamertag, box owner, or a room.join scope)
host:<instance>:<class>  per-class room — scope for machine keys; scope + matching binding for spectator / device / anonymous
host:all, host:summary   aggregate feeds — room.join scope (admin role, machine keys)
admin:<name>             admin.admin scope, pb_user only
public:<name>, <other>   any registered type — every kind but discord, credential or not
```

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
