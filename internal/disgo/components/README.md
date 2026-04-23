# internal/disgo/components

Styled UI factory functions for Discord message components.

## Key Concept

Components are **builders, not handlers**. Each file exports a pure function that returns a styled Discord UI element. The interaction handler for that element lives in the **command file** that created it — not here.

## Subdirectories

| Directory  | Purpose                              | Example                                      |
|------------|--------------------------------------|----------------------------------------------|
| `buttons/` | Individual button builders           | `Confirm(customID) discord.ButtonComponent`  |
| `embeds/`  | Embed builders (success, error, etc) | `Success(title, desc) discord.Embed`         |
| `rows/`    | Action row composers                 | `ConfirmRow(yesID, noID) discord.ActionRowComponent` |
| `selects/` | Select menu builders                 | *(future)*                                   |
| `modals/`  | Modal builders                       | *(future)*                                   |

## One File, One Component

Each `.go` file exports a single builder function. No `init()`, no registry — just a pure function.

## How Custom ID Routing Works

Components that trigger interactions (buttons, selects, modals) carry a **custom ID**. The `handler.Mux` in `bot.go` routes interactions by matching that custom ID as a path.

1. A **component builder** here sets the custom ID (passed as a parameter)
2. A **command file** in `commands/` registers the handler for that custom ID on the mux

```
// components/buttons/confirm.go — builds a styled button
buttons.Confirm("/create-channel/confirm")

// commands/create_channel.go — handles the button click
mux.ButtonComponent("/create-channel/confirm", handleConfirm)
```

The builder owns the **style**. The command owns the **behavior**.

## Adding a New Component

1. Create a file in the appropriate subdirectory (e.g., `buttons/retry.go`)
2. Export one function that returns the Discord type (e.g., `discord.ButtonComponent`)
3. Use it in any command handler — no registration needed
