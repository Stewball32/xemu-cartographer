# internal/disgo

Discord bot implementation using [Disgo](https://github.com/disgoorg/disgo).

## Responsibilities

Creates and manages the Disgo bot client. Registers slash commands and attaches event listeners. The bot runs alongside PocketBase, started in the `OnServe` hook and shut down in `OnTerminate`.

## Subdirectories

| Directory      | Purpose                                                              |
|----------------|----------------------------------------------------------------------|
| `commands/`    | Slash command definitions and interaction handlers (self-registering) |
| `events/`      | Discord gateway event listeners (self-registering)                   |
| `actions/`     | Reusable Discord API calls — one exported function per file          |
| `resolvers/`   | Discord data lookups via `*guards.Services` — one function per file  |
| `components/`  | UI builder factories (buttons, embeds, rows, selects, modals)        |
| `guards/`      | Bot-side permission checks bridging Discord ↔ PocketBase             |

## Key Files

- `bot.go` — `Bot` struct, `NewBot()`, `OpenGateway()`, `Close()`, package-level `Instance()` accessor, action wrapper methods (`SendNotification()`, `CreateVoiceChannel()`) that satisfy `discordiface.Service`, and `SetServices()`/`Services()` for cross-system access
- `commands/allcommands.go` — `Command` struct + registry (`register()` / `All()`), `SetServices()` for cross-system access from command handlers
- `events/allevents.go` — event listener registry (`register()` / `RegisterAll()`)

## Bot Lifecycle

The bot is wired into PocketBase's lifecycle in `cmd/server/main.go`:

```go
// In OnServe:
bot, err = discordbot.NewBot()        // builds client, syncs commands
bot.OpenGateway(context.Background()) // connects to Discord gateway (non-blocking)
discordbot.SetInstance(bot)           // makes bot accessible via disgo.Instance()
bot.SetServices(svc)                  // cross-system access (PB, WS, Discord)
commands.SetServices(svc)             // makes Services available to command handlers

// In OnTerminate:
bot.Close(context.Background())
```

Startup is non-fatal — if `DISCORD_BOT_TOKEN` is missing, the server logs a warning and continues without the bot.

## Cross-System Access

Command handlers access other systems via `commands.services()`:

```go
func handleLookup(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
    svc := services()

    // Resolve from PocketBase
    user, err := svc.PB.FindUserByDiscordID(data.User().ID.String())

    // Check WebSocket state
    if svc.WS != nil && svc.WS.IsConnected(user.Id) { ... }

    // Broadcast via WebSocket
    svc.WS.BroadcastRaw(msgBytes)
}
```

The `Bot` struct also implements `discordiface.Service` — its action methods (`SendNotification()`, `CreateVoiceChannel()`) wrap standalone functions from `actions/`, making them callable through the `Services` interface from any system.

## Design Principles

- **One file, one thing** — every command, guard, button, embed, action is its own `.go` file
- **Triggers vs actions** — commands and event listeners are triggers; reusable Discord API calls live in `actions/`
- **Component builders, not component handlers** — `components/` holds styled UI factory functions; interaction handlers stay with the command that created them
- **Guards, not middleware** — Discord doesn't have middleware; guards are explicit checks called at the top of command handlers
- **Portability** — copy a file to another project, IDE flags missing deps, done

## Adding New Items

### Slash command (self-registering)

1. Create a new file in `commands/` (e.g., `stats.go`)
2. Add an `init()` function that calls `register(Command{...})` with your `SlashCommandCreate` definition and handler
3. Done — `bot.go` picks it up automatically via `commands.All()`

### Event listener (self-registering)

1. Create a new file in `events/` (e.g., `member_join.go`)
2. Add an `init()` function that calls `register()` with a function that adds your listener
3. Done — `bot.go` calls `events.RegisterAll(client)` automatically

### Action (no registry)

1. Create a new file in `actions/` (e.g., `send_dm.go`)
2. Export a single function taking `*bot.Client` as the first parameter
3. Call it from any trigger: `actions.SendDM(disgo.Instance().Client, ...)`

### Component (no registry)

1. Create a new file in the appropriate `components/` subdirectory
2. Export a pure builder function (e.g., `buttons.Confirm(customID) discord.ButtonComponent`)
3. Use it in command handlers to build styled UI elements

### Guard (no registry)

1. Create a new file in `guards/` (e.g., `require_guild_owner.go`)
2. Export a single function that takes the interaction event, checks a condition, returns an error on failure
3. Call it explicitly at the top of command handlers
