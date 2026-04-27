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
	scraperroutes "github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/schema"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/seed"
	"github.com/Stewball32/xemu-cartographer/internal/podman"
	scrapermgr "github.com/Stewball32/xemu-cartographer/internal/scraper/manager"
	ws "github.com/Stewball32/xemu-cartographer/internal/websocket"

	discordbot "github.com/Stewball32/xemu-cartographer/internal/disgo"
	"github.com/Stewball32/xemu-cartographer/internal/disgo/commands"
	pb "github.com/Stewball32/xemu-cartographer/internal/pocketbase"
	_ "github.com/Stewball32/xemu-cartographer/internal/scraper/haloce"      // self-registering Halo: CE GameReader
	_ "github.com/Stewball32/xemu-cartographer/internal/websocket/handlers"  // self-registering WS handlers
	_ "github.com/Stewball32/xemu-cartographer/internal/websocket/rooms"     // self-registering WS room types
)

func main() {
	app := pocketbase.New()

	var bot *discordbot.Bot
	var hub *ws.Hub
	var watcherCancel context.CancelFunc
	var scrMgr *scrapermgr.Manager

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

		// Build the Services skeleton early so subsystems that need to broadcast
		// (the scraper manager) can hold a stable pointer to it. Per-system
		// fields (svc.WS, svc.Discord) are populated as those subsystems come up
		// later in this OnServe block — Go's pointer semantics mean the scraper
		// sees the live values without needing a SetServices callback.
		pbSvc := pb.NewService(app)
		svc := &guards.Services{
			App: app,
			PB:  pbSvc,
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

		// Scraper manager: always available. Holds a *Services pointer; broadcasts
		// safely no-op until svc.WS is populated below. The blank import of
		// internal/scraper/haloce above triggers haloce.init(), which registers
		// Halo: CE's title ID with scraper.Lookup so manager.Start() can detect it.
		scrMgr = scrapermgr.New(svc)
		svc.Scraper = scrMgr
		scraperroutes.SetManager(scrMgr)

		routes.RegisterAll(se)

		hub = ws.NewHub(app)
		go hub.Run()
		ws.SetInstance(hub)
		se.Router.GET("/api/ws", ws.NewHandler(hub, app))
		svc.WS = hub
		hub.SetServices(svc)

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
				svc.Discord = bot
				bot.SetServices(svc)
			}
		}

		hooks.SetServices(svc)
		commands.SetServices(svc)

		return se.Next()
	})

	// OnTerminate: cleanup.
	app.OnTerminate().BindFunc(func(te *core.TerminateEvent) error {
		if watcherCancel != nil {
			watcherCancel()
		}

		// Stop scrapers BEFORE the hub so in-flight tick broadcasts don't try to
		// write to a closing channel. Manager.Stop blocks until each runner's
		// tick goroutine exits.
		if scrMgr != nil {
			for _, info := range scrMgr.List() {
				if err := scrMgr.Stop(info.Name); err != nil {
					log.Printf("scraper: stop %s on shutdown: %v", info.Name, err)
				}
			}
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
