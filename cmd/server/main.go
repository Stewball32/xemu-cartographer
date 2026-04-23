package main

import (
	"context"
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/youruser/yourproject/internal/guards"
	"github.com/youruser/yourproject/internal/pocketbase/hooks"
	"github.com/youruser/yourproject/internal/pocketbase/oauth"
	"github.com/youruser/yourproject/internal/pocketbase/routes"
	"github.com/youruser/yourproject/internal/pocketbase/schema"
	"github.com/youruser/yourproject/internal/pocketbase/seed"
	ws "github.com/youruser/yourproject/internal/websocket"

	discordbot "github.com/youruser/yourproject/internal/disgo"
	"github.com/youruser/yourproject/internal/disgo/commands"
	pb "github.com/youruser/yourproject/internal/pocketbase"
	_ "github.com/youruser/yourproject/internal/websocket/handlers" // self-registering WS handlers
	_ "github.com/youruser/yourproject/internal/websocket/rooms"    // self-registering WS room types
)

func main() {
	app := pocketbase.New()

	var bot *discordbot.Bot
	var hub *ws.Hub

	// Register record lifecycle hooks (callback registration, fires later).
	hooks.RegisterAll(app)

	// OnServe: register schemas and routes (needs running DB / ServeEvent).
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		if err := schema.RegisterAll(app); err != nil {
			return err
		}

		if err := oauth.RegisterAll(app); err != nil {
			return err
		}

		if err := seed.Run(app); err != nil {
			return err
		}

		routes.RegisterAll(se)

		hub = ws.NewHub(app)
		go hub.Run()
		ws.SetInstance(hub)
		se.Router.GET("/api/ws", ws.NewHandler(hub, app))

		// Start Disgo bot (non-blocking)
		var err error
		bot, err = discordbot.NewBot()
		if err != nil {
			log.Printf("Warning: Discord bot not started: %v", err)
		} else {
			if err := bot.OpenGateway(context.Background()); err != nil {
				log.Printf("Warning: Discord gateway failed: %v", err)
				bot = nil
			} else {
				discordbot.SetInstance(bot)
			}
		}

		// Wire up cross-system Services for guards, resolvers, and actions.
		pbSvc := pb.NewService(app)
		svc := &guards.Services{
			App: app,
			WS:  hub,
			PB:  pbSvc,
		}
		if bot != nil {
			svc.Discord = bot
			bot.SetServices(svc)
		}
		hub.SetServices(svc)
		hooks.SetServices(svc)
		commands.SetServices(svc)

		return se.Next()
	})

	// OnTerminate: cleanup.
	app.OnTerminate().BindFunc(func(te *core.TerminateEvent) error {
		if hub != nil {
			hub.Stop()
		}

		if bot != nil {
			bot.Close(context.Background())
		}

		log.Println("Server shutting down...")
		return te.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
