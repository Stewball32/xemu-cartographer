package events

import "github.com/disgoorg/disgo/bot"

var registry []func(client *bot.Client)

// register adds an event listener registration function.
// Call this from init() in each event domain file.
func register(fn func(client *bot.Client)) {
	registry = append(registry, fn)
}

// RegisterAll attaches all event listeners to the bot client.
// Called by bot.go after client creation.
func RegisterAll(client *bot.Client) {
	for _, fn := range registry {
		fn(client)
	}
}
