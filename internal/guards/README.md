# internal/guards

Unified cross-system guards, resolvers, and dependency injection for the three main systems (PocketBase, Disgo, WebSocket).

## Key Files

- `services.go` — `Services` struct bundling all system interfaces (`Discord`, `WS`, `PB`, `App`). Fields are nil if the corresponding system is not running.
- `guard.go` — `GuardFunc` type: `func(svc *Services, user *core.Record) error`
- `compose.go` — `Any()` and `All()` combinators for composing multiple guards
- `errors.go` — Shared error values (`ErrAuthRequired`, `ErrForbidden`)
- `require_*.go` — Guard implementations (one per file)

## Interfaces

Per-system subdirectories in `interfaces/` with one interface per file for merge-safe parallel development:

```
interfaces/
├── discord/
│   ├── membership.go    # Membership { IsMember() }
│   ├── roles.go         # Roles { MemberRoles() }
│   ├── notify.go        # Notify { SendNotification() }
│   ├── voice.go         # Voice { CreateVoiceChannel() }
│   └── discord.go       # Service = Membership + Roles + Notify + Voice
├── websocket/
│   ├── connected.go     # Connected { IsConnected() }
│   ├── rooms.go         # Rooms { IsInRoom(), UserRooms() }
│   ├── broadcast.go     # Broadcast { BroadcastRaw(), SendToUserRaw(), SendToRoomRaw() }
│   └── websocket.go     # Service = Connected + Rooms + Broadcast
├── pocketbase/
│   ├── users.go         # Users { FindUserByDiscordID() }
│   ├── gamertags.go     # Gamertags { FindGamertagsForUser(), FindUserByGamertagString(), FindActiveRostersForUser() }  (M07)
│   └── pocketbase.go    # Service = Users + Gamertags + ...
└── scraper/
    ├── lifecycle.go     # Lifecycle { Start(name, sock), Stop(name) }
    ├── inspect.go       # Inspect { List(), Inspect(name) } + Info / InspectState / PreviousGameInfo structs
    ├── joinreplay.go    # JoinReplay { JoinReplayMessages(), JoinReplayForInstance(name), JoinReplayForInstanceClass(name, class), JoinReplayForHostAll() }
    ├── events_reply.go  # EventsReply { EventsReply(instance, sinceTick, types) }
    ├── probe_reply.go   # ProbeReply { ProbeReply(instance) }
    ├── state.go         # State { InstanceState(name) } + InstanceState struct
    └── scraper.go       # Service = Lifecycle + Inspect + JoinReplay + EventsReply + ProbeReply + State
```

Small interfaces compose into aggregate `Service` interfaces via embedding. Each system's concrete type (`disgo.Bot`, `websocket.Hub`, `pocketbase.Service`) implements the aggregate interface via Go's structural typing — no explicit import of the interface package needed.

## How It Works

1. `main.go` builds the `Services` struct with concrete implementations
2. Injects it into all three systems via `SetServices()` calls
3. Handlers from any system access cross-system functionality through `*Services`
4. Guards check permissions, resolvers look up data, actions execute operations — all through the same `*Services` reference

## Adding a New Guard

1. Create a new file (e.g., `require_verified.go`)
2. Export a `GuardFunc` or a factory returning one
3. Use `svc.Discord`, `svc.WS`, `svc.PB`, or `svc.App` to check conditions
4. Return `nil` to pass, or an error to deny

## Adding a New Interface Method

1. Create a new file in the appropriate `interfaces/` subdirectory (e.g., `interfaces/discord/ban.go`)
2. Define a single-method interface (e.g., `type Ban interface { BanMember(...) error }`)
3. Embed it in the aggregate `Service` interface in the subdirectory's root file (e.g., `interfaces/discord/discord.go`)
4. Implement the method on the concrete type (`disgo.Bot`, `websocket.Hub`, or `pocketbase.Service`)
