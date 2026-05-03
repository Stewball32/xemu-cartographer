# WebSocket

The WebSocket subsystem is a single hub goroutine plus one `Client` per browser tab (each with its own read+write goroutines). It hands real-time scraper traffic to the overlay UI, lets PocketBase hooks and Disgo push notifications to specific users, and gates room membership behind cross-system guards. Everything below is about the hub *as it exists today* — no recommendations, just behavior.

> Where it lives: [internal/websocket/](../internal/websocket/) (hub, clients, handlers, rooms) and [internal/guards/interfaces/websocket/](../internal/guards/interfaces/websocket/) (cross-system interface). The scraper integration that produces most of the traffic is in [internal/scraper/manager/loop.go](../internal/scraper/manager/loop.go).

---

## 1. Lifecycle: when does the hub start and stop?

There is exactly one hub per server, and every connection is a `Client` registered with that hub.

### Start

In `OnServe` ([cmd/server/main.go:146-151](../cmd/server/main.go#L146)):

```go
hub = ws.NewHub(app)
go hub.Run()
ws.SetInstance(hub)
se.Router.GET("/api/ws", ws.NewHandler(hub, app))
svc.WS = hub
hub.SetServices(svc)
```

The scraper manager is built earlier, at [main.go:77-78](../cmd/server/main.go#L77), holding the same `*Services` pointer. `svc.WS` is nil between then and line 150. The scraper's `broadcast` short-circuits when `svc.WS == nil` ([loop.go:142-144](../internal/scraper/manager/loop.go#L142)), so any tick reads that happen during that ~ms window are silently no-op'd rather than crashing.

### Stop

`OnTerminate` ([main.go:176-202](../cmd/server/main.go#L176)) runs in this order:

1. Cancel the discovery watcher (so no new scrapers are auto-started during teardown).
2. Stop every scraper runner — `Manager.Stop` blocks on `<-r.done`, so by the time it returns the runner's tick goroutine is gone and won't broadcast again.
3. `hub.Stop()`.
4. Close the Disgo bot.

Step 2 *must* precede step 3: if the hub closes a client's send channel while a scraper goroutine is mid-`SendToRoomRaw`, the `trySend` could write to a closing channel. Stopping scrapers first guarantees no producer is racing the hub's shutdown.

`hub.Stop()` is `sync.Once`-guarded and just `close(h.done)` ([hub.go:258-260](../internal/websocket/hub.go#L258)). The `case <-h.done` in the Run loop ([hub.go:120-127](../internal/websocket/hub.go#L120)) takes the write lock, closes every `client.send`, and returns. Each client's `writePump` sees the closed channel, exits, and closes the underlying connection.

---

## 2. The connection upgrade

One `Client` per `GET /api/ws` ([handler.go](../internal/websocket/handler.go)).

- **Auth.** Optional `?token=` query param. The handler calls `app.FindAuthRecordByToken(token, core.TokenTypeAuth)` ([handler.go:22-29](../internal/websocket/handler.go#L22)). If the token is missing OR invalid, the failure is **logged and the connection proceeds anonymously** — `client.user == nil`, `UserID() == ""`. There is no client-visible signal that auth failed at the handshake; the client only finds out when a guarded operation (e.g. `join_room overlay`) returns `{"type":"error","payload":{"code":"forbidden",...}}`.
- **Origin policy.** `WS_ALLOWED_ORIGINS` is comma-split into `OriginPatterns` ([handler.go:54-72](../internal/websocket/handler.go#L54)). If unset, `InsecureSkipVerify: true` accepts any origin (dev convenience).
- **Goroutines per client.** `writePump` is spun off; `readPump` runs **on the request goroutine itself** and blocks until disconnect ([handler.go:46-47](../internal/websocket/handler.go#L46)). When `readPump` returns, its deferred `c.hub.unregister <- c` schedules cleanup and `c.conn.Close` fires ([client.go:38-41](../internal/websocket/client.go#L38)).
- **Read limits.** `SetReadLimit(4096)` ([client.go:43](../internal/websocket/client.go#L43)). Messages larger than 4 KiB cause `Read` to error and the pump to exit, taking the connection with it. JSON parse errors on smaller messages are logged but the pump continues ([client.go:54-58](../internal/websocket/client.go#L54)).

---

## 3. The Run() event loop — the single serialization point

`Hub.Run` is one goroutine selecting on six channels ([hub.go:80-129](../internal/websocket/hub.go#L80)):

| Channel | Action | Lock |
| --- | --- | --- |
| `register` | Add to `h.clients`; if authed, also `h.users[uid]` | `mu.Lock` |
| `unregister` | Delegate to `removeClient` | `mu.Lock` (in `removeClient`) |
| `incoming` | Call `dispatch(im)` — no lock held | none directly; handlers may grab it via closures |
| `joinRoom` | Add to `h.rooms[name]`, creating the map if needed | `mu.Lock` |
| `leaveRoom` | Remove from `h.rooms[name]`; delete the room if empty | `mu.Lock` |
| `done` | Close every `client.send` and return | `mu.Lock` |

`dispatch` ([hub.go:132-139](../internal/websocket/hub.go#L132)):

```go
if handler, ok := handlers.Get(im.msg.Type); ok {
    handler(h.buildEvent(im))
    return
}
// No registered handler — default to broadcast.
h.broadcast(im.msg)
```

That last line is load-bearing: **any unknown `type` from any client is rebroadcast to every connected client as-is.** A buggy or malicious client sending `{"type":"anything"}` will fan that message out to everyone.

The `Event` passed to a handler ([handlers/allhandlers.go:13-31](../internal/websocket/handlers/allhandlers.go#L13)) carries the sender's identity, the message metadata, and a set of closures back into the hub: `Broadcast`, `SendToRoom`, `SendToUser`, `SendRaw`, `SendError`, `JoinRoom`, `LeaveRoom` ([hub.go:142-191](../internal/websocket/hub.go#L142)). The `JoinRoom`/`LeaveRoom` closures take `h.mu.Lock()` directly inside the handler call. Since the Run loop is the single dispatcher, this is safe — but it does mean a handler that grabs the lock blocks the loop from servicing any other channel until it's done.

Handlers are **synchronous on the Run goroutine**. There's no per-handler timeout, no goroutine isolation, no recover. A slow handler stalls the entire hub.

---

## 4. Inbound message flow

Wire format ([message.go](../internal/websocket/message.go)):

```go
type Message struct {
    Type    string          `json:"type"`
    Room    string          `json:"room,omitempty"`
    Target  string          `json:"target,omitempty"`
    Payload json.RawMessage `json:"payload,omitempty"`
}
```

`Payload` is opaque — the hub never parses it.

Path: `readPump` decodes JSON → `c.hub.incoming <- {msg, c}`. `incoming` is a 256-slot buffered channel ([hub.go:66](../internal/websocket/hub.go#L66)). If the buffer fills (because the Run loop is stuck in a slow handler), `readPump`'s send blocks, which blocks reads from that client's socket. That's per-client backpressure for that one tab — but if the slow handler is in *another* client's call, every tab's reads stall together.

Built-in handlers: `join_room`, `leave_room`. That's the entire registry. Anything else falls into the default-broadcast in §3.

### `join_room`

[handlers/join_room.go](../internal/websocket/handlers/join_room.go):

1. If `e.Room` is empty, return.
2. `rooms.Resolve(e.Room)` parses the room name's `type:` prefix → look up `RoomType`. Unknown prefix → `SendError("not_found", "unknown room type")`.
3. `rt.CheckGuards(e.Services, e.User)` — first error wins → `SendError("forbidden", err.Error())`.
4. `e.JoinRoom(e.Room)`.
5. **Overlay-only:** if the room name's type is `overlay` and `svc.Scraper` is wired in, iterate `svc.Scraper.LatestSnapshotMessages()` and `SendRaw` each cached snapshot to the joiner ([handlers/join_room.go:30-34](../internal/websocket/handlers/join_room.go#L30)). This is the *only* mechanism mid-match joiners use to get map/players/power-item-spawn data — without it, a joiner sits without state until the next game-state transition fires a fresh snapshot, which can be the entire match.

### `leave_room`

[handlers/leave_room.go](../internal/websocket/handlers/leave_room.go) is just `e.LeaveRoom(e.Room)` if room is non-empty. No guards, no replay logic.

---

## 5. Outbound message flow and backpressure

There are two API tiers, with very different concurrency profiles.

### Typed APIs — go through `incoming`

[hub.go:196-213](../internal/websocket/hub.go#L196):

```go
func (h *Hub) Broadcast(msg Message) {
    msg.Type = TypeBroadcast
    h.incoming <- incomingMsg{msg: msg}
}
```

`Broadcast`, `SendToUser`, `SendToRoom` all enqueue an `incomingMsg` with `sender == nil`. The Run loop pulls it, dispatches → no handler matches `TypeBroadcast`/`TypeRoom`/`TypeDirect`, so it falls through to the default broadcast in `dispatch` (which routes by `msg.Type` in the inner `broadcast`/`sendToRoom`/`sendToUser` helpers). **They share the 256-slot `incoming` buffer with all inbound client traffic.** A flood of either direction can fill it.

### Raw APIs — bypass the loop

[hub.go:217-255](../internal/websocket/hub.go#L217):

```go
func (h *Hub) SendToRoomRaw(room string, data []byte) {
    h.mu.RLock()
    clients := make([]*Client, 0, len(h.rooms[room]))
    for client := range h.rooms[room] {
        clients = append(clients, client)
    }
    h.mu.RUnlock()
    for _, client := range clients {
        h.trySend(client, data)
    }
}
```

The pattern is **snapshot under RLock → release → trySend each client outside the lock.** `BroadcastRaw` and `SendToUserRaw` follow the same shape. These are what the scraper, hooks, and Disgo use through `svc.WS` — they never touch `h.incoming`.

### `trySend` — the silent-drop and async-evict primitive

[hub.go:360-367](../internal/websocket/hub.go#L360):

```go
func (h *Hub) trySend(client *Client, data []byte) {
    select {
    case client.send <- data:
    default:
        // Defer removal to avoid lock contention — Run() handles unregister.
        go func() { h.unregister <- client }()
    }
}
```

If the client's 256-slot send buffer is full, the message is dropped silently and a goroutine is spawned to evict the client. The drop is per-message: every subsequent `trySend` to that client during the eviction window also drops, also silently. The eventual disconnect produces only a generic "read error" / "write error" log line on the way out — there's no metric and no marker on the broadcast side correlating drops to a specific client or room.

At 30 Hz scraper ticks, 256 slots is ~8 seconds of slack before a slow tab is disconnected.

### `writePump`

[client.go:66-86](../internal/websocket/client.go#L66): reads from `c.send`, writes with a 10 s per-write context timeout. Any write error or context cancel exits the pump and closes the connection. When `removeClient` closes `c.send`, the pump receives `ok == false` and exits cleanly.

### `removeClient`

[hub.go:370-390](../internal/websocket/hub.go#L370): under `mu.Lock`, removes the client from every room (pruning empty rooms), from `users[uid]` (pruning the user entry if empty), from `clients`, then `close(client.send)`.

---

## 6. Rooms

A `RoomType` is a category of rooms with a guard list and an optional `MaxMembers` ([rooms/registry.go:14-25](../internal/websocket/rooms/registry.go#L14)). `Config.MaxMembers` is currently dead — no code reads it.

Room *instances* are keys in `hub.rooms`. They're created on first join ([hub.go:104-106](../internal/websocket/hub.go#L104)) and deleted when the last member leaves ([hub.go:111-117](../internal/websocket/hub.go#L111)). There's no per-room state, no pre-allocation, no separate locks.

Room name → type resolution ([rooms/registry.go:37-44](../internal/websocket/rooms/registry.go#L37)) splits on the first `:` and looks up the prefix in the registry. So `"overlay"`, `"overlay:foo"`, `"overlay:bar"` all resolve to the `overlay` `RoomType` and run the same guards, but they live as **three separate rooms** in `hub.rooms`. A broadcast to `"overlay"` does not reach members of `"overlay:foo"`.

Registered room types — all self-register via `init()` on package import ([cmd/server/main.go:30](../cmd/server/main.go#L30) blank-imports `internal/websocket/rooms` to wire them in):

| Type | Guards | File |
| --- | --- | --- |
| `admin` | `RequireAdmin` | [rooms/admin.go](../internal/websocket/rooms/admin.go) |
| `overlay` | `RequireAuth` | [rooms/overlay.go](../internal/websocket/rooms/overlay.go) |
| `public` | none | [rooms/public.go](../internal/websocket/rooms/public.go) |

A client can be in any number of rooms simultaneously — memberships are independent map entries.

---

## 7. Cross-system service interface

[internal/guards/interfaces/websocket/](../internal/guards/interfaces/websocket/) splits the hub's surface into `Connected`, `Rooms`, and `Broadcast`, and aggregates them into `Service`. The hub satisfies it structurally — no explicit `implements` anywhere. Cross-system code (scraper manager, PocketBase hooks, Disgo handlers) talks to the hub through `svc.WS` so imports stay one-way (scraper → guards → ws-iface, never scraper → ws).

[`ws.Instance()`](../internal/websocket/hub.go#L55) is also exposed for legacy/internal use, but the convention is: external producers go through `svc.WS`.

`IsConnected`, `IsInRoom`, `UserRooms` all take `mu.RLock` ([hub.go:265-301](../internal/websocket/hub.go#L265)) so they're safe to call from anywhere.

---

## 8. Scraper integration

The scraper manager is the only large external producer today. Per-runner `broadcast` ([loop.go:141-170](../internal/scraper/manager/loop.go#L141)):

1. Marshal the `scraper.Envelope` (one of `snapshot` / `tick` / `event`).
2. Wrap it in `Message{Type:"scraper", Room:"overlay", Payload:envBytes}`.
3. Marshal the wrapping `Message`.
4. **Cache** depending on envelope type:
   - `snapshot` → `r.latestSnapshotMsg` (the wrapped wire bytes, not the parsed envelope) under `r.cacheMu`.
   - `event` → push onto `r.recentEvents`, ring buffer capped at 50 ([runner.go:53](../internal/scraper/manager/runner.go#L53)).
   - `tick` → **not cached as wire bytes.** Only the unwrapped tick payload is cached for the inspect HTTP endpoint, never replayed.
5. `svc.WS.SendToRoomRaw("overlay", msgBytes)`.

"Wrap-not-extend": every envelope rides inside the same outer `Message` schema as everything else. The frontend parses two layers — the outer `Message`, then `msg.payload` as a `scraper.Envelope`.

### Late-join replay

[`Manager.LatestSnapshotMessages()`](../internal/scraper/manager/manager.go#L196) snapshots the runner list under `m.mu`, then per-runner reads the cached `r.latestSnapshotMsg` under `r.cacheMu` and copies the bytes. Each returned `[]byte` is a fresh copy, safe to send without further locking. The `join_room` handler walks this list and sends one message per runner that has emitted at least one snapshot.

**Tick and event envelopes are not replayed.** A client that joins `overlay` mid-match gets:

- The most recent snapshot from each running scraper (replayed via `SendRaw` immediately after `JoinRoom`).
- All future ticks/events broadcast to the overlay room from now on.

Anything that happened in the gap between the previous snapshot and the join is lost.

---

## 9. Frontend consumer

There is one WebSocket client in the frontend: [scraper-ws.svelte.ts](../sveltekit/src/lib/stores/scraper-ws.svelte.ts), exported as the singleton `scraperWS`.

- **URL:** `${wsBaseURL()}/api/ws?token=${encodeURIComponent(token)}`. Token is the PocketBase JWT from the auth store.
- **On `open`:** sets `connected = true`, sends `{"type":"join_room","room":"overlay"}` ([scraper-ws.svelte.ts:78-83](../sveltekit/src/lib/stores/scraper-ws.svelte.ts#L78)).
- **On `message`:** parses outer `Message`. If `msg.type === "scraper"`, the payload is treated as `Envelope` and routed by `env.type` to `snapshots` / `ticks` / `events`, all keyed by `env.instance`. If `msg.type === "error"`, the payload's `message` lands in `lastError`.
- **Reconnect ladder:** `[1000, 2000, 4000, 8000, 15000, 30000]` ms, no jitter. `manuallyClosed` flag suppresses reconnect when the user explicitly disconnects.
- **Token freshness:** the store does not refresh the token before reconnecting. If the JWT expires while the tab is open, the next reconnect will upgrade anonymously (per §2) and the auto-`join_room overlay` will return `forbidden`.

Consumers:

| Page | What it reads |
| --- | --- |
| [overlays/players/+page.svelte](../sveltekit/src/routes/overlays/players/+page.svelte) | `scraperWS.snapshot` / `scraperWS.tick` (first-instance convenience getters) |
| [admin/debug/[name]/+page.svelte](../sveltekit/src/routes/admin/debug/[name]/+page.svelte) | `scraperWS.snapshots[name]` / `ticks[name]` / `events[name]`, falling back to the REST `inspect` endpoint when WS hasn't delivered yet |

---

## 10. Concurrency & lock summary

| Object | Owner / Goroutine | Lock | Notes |
| --- | --- | --- | --- |
| `hub.clients`, `hub.users`, `hub.rooms` | Run loop writes; raw-API callers + readers RLock | `hub.mu` (RWMutex) | Run loop takes write lock briefly; raw APIs snapshot the slice and release before `trySend` |
| `client.send` | `trySend` writes (non-blocking); `writePump` reads | none | 256-slot buffer; full → drop + async evict |
| `client.user`, `client.conn` | set once at construction | none | read-only after that |
| `hub.incoming` | `readPump`s + typed-API callers write; Run loop reads | none | 256-slot buffer; **shared between inbound and typed-outbound** |
| `r.latestSnapshotMsg`, `r.recentEvents` | tick goroutine writes; `LatestSnapshotMessages` / inspect read | `r.cacheMu` | Each `LatestSnapshotMessages` call returns fresh byte copies |

The Run goroutine is the only writer for `clients`/`users`/`rooms`. Any reader (typed APIs, raw APIs, `IsConnected`, etc.) takes an `RLock` and snapshots before doing real work.

---

## 11. Things to be aware of

Behavioral observations, not recommendations.

- **Default-broadcast on unknown type.** Any client message with a `type` not in the handler registry is broadcast to every connected client as-is ([hub.go:138](../internal/websocket/hub.go#L138)). Built-in registry is just `join_room` and `leave_room`.
- **Send-buffer drops are silent.** A slow tab loses messages and gets evicted with only a generic read/write error log line on the way out ([hub.go:360-367](../internal/websocket/hub.go#L360)). No metric, no broadcast-side correlation, nothing observable from the producer's side.
- **Late-join snapshot vs. live broadcast race.** `join_room` adds the client to the room and *then* replays cached snapshots ([handlers/join_room.go:25-34](../internal/websocket/handlers/join_room.go#L25)). Between those two operations a fresh tick or snapshot can be broadcast to the room. The new joiner can therefore see: live tick → cached (older) snapshot → more live ticks. The frontend keys snapshot/tick by instance, so the cached snapshot's payload overwrites whatever the just-arrived tick wrote — possibly making the UI look briefly stale right after a join.
- **`incoming` buffer is shared between inbound and typed-outbound.** `Broadcast` / `SendToUser` / `SendToRoom` all enqueue onto the same 256-slot channel that `readPump`s feed. Raw APIs bypass it; typed APIs don't.
- **Handler synchronicity stalls everything.** Handlers run on the Run goroutine. A handler that touches the database with no timeout blocks every other channel — no other client can register, unregister, join, leave, or send a typed broadcast until it returns. Raw API callers (the scraper) are unaffected because they don't go through Run.
- **Anonymous connect is allowed but invisible.** An invalid or expired JWT is logged on the server side and the connection upgrades anonymously ([handler.go:22-29](../internal/websocket/handler.go#L22)). The client only learns about it when a guarded operation rejects them. The frontend store stuffs that into `lastError` but doesn't refresh the token or reconnect.
- **Tick and event envelopes have no replay.** Only snapshots are cached as wire bytes. A reconnect during a match brings the joiner up to date on the most recent snapshot, but the next live tick is the first thing they see — events that fired in the gap are lost.
- **Room name colon-prefix is shared, instances are not.** `"overlay"`, `"overlay:foo"`, `"overlay:bar"` all run the same guards but live as separate rooms. A broadcast to `"overlay"` does not reach members of `"overlay:foo"`. The scraper currently broadcasts to the bare string `"overlay"` ([loop.go:27](../internal/scraper/manager/loop.go#L27)).
- **`Config.MaxMembers` is dead config.** Defined in the `RoomType` config struct but never read.
