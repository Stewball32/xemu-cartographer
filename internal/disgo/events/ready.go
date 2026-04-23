package events

import (
	"log"

	"github.com/disgoorg/disgo/bot"
	disgoevents "github.com/disgoorg/disgo/events"
)

// Registration: queues this listener to be attached when RegisterAll is called.
func init() {
	register(registerReadyListener)
}

// Listener: attach event handlers to the client here.
func registerReadyListener(client *bot.Client) {
	client.AddEventListeners(bot.NewListenerFunc(func(e *disgoevents.Ready) {
		log.Printf("Discord bot ready as %s", e.User.Username)
	}))
}
