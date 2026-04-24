package main

import (
	"context"
	"log"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/Stewball32/xemu-cartographer/internal/discovery"
	"github.com/Stewball32/xemu-cartographer/internal/guards"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/hooks"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/oauth"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/containers"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/schema"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/seed"
	"github.com/Stewball32/xemu-cartographer/internal/podman"
	ws "github.com/Stewball32/xemu-cartographer/internal/websocket"

	discordbot "github.com/Stewball32/xemu-cartographer/internal/disgo"
	"github.com/Stewball32/xemu-cartographer/internal/disgo/commands"
	pb "github.com/Stewball32/xemu-cartographer/internal/pocketbase"
	_ "github.com/Stewball32/xemu-cartographer/internal/websocket/handlers" // self-registering WS handlers
	_ "github.com/Stewball32/xemu-cartographer/internal/websocket/rooms"    // self-registering WS room types
)

func main() {
	app := pocketbase.New()

	var bot *discordbot.Bot
	var hub *ws.Hub
	var watcherCancel context.CancelFunc

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

		// Containers (optional): start podman manager + socket watcher when
		// CONTAINERS_ENABLED=true. The route group registers itself as a
		// no-op when Manager is nil, so a fresh checkout boots cleanly.
		podmanCfg := podman.LoadFromEnv()
		if podmanCfg.Enabled {
			mgr, err := podman.NewManager(podmanCfg)
			if err != nil {
				return err
			}
			containers.SetManager(mgr)

			if podmanCfg.SocketDir != "" {
				ctx, cancel := context.WithCancel(context.Background())
				watcherCancel = cancel
				w := discovery.NewWatcher(podmanCfg.SocketDir, 2*time.Second,
					func(name, sock string) {
						log.Printf("discovery: socket up name=%s path=%s", name, sock)
					},
					func(name string) {
						log.Printf("discovery: socket down name=%s", name)
					},
				)
				go w.Run(ctx)
			}
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
		if watcherCancel != nil {
			watcherCancel()
		}

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
