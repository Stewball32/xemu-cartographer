package main

import (
	"context"
	"crypto/rand"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/discovery"
	"github.com/Stewball32/xemu-cartographer/internal/guards"
	"github.com/Stewball32/xemu-cartographer/internal/hostrunner"
	"github.com/Stewball32/xemu-cartographer/internal/overlaytoken"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/hooks"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/oauth"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/resolvers"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/containers"
	overlaysroutes "github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/overlays"
	playroutes "github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/play"
	scraperroutes "github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/schema"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/seed"
	"github.com/Stewball32/xemu-cartographer/internal/podman"
	scrapermgr "github.com/Stewball32/xemu-cartographer/internal/scraper/manager"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/sinks"
	ws "github.com/Stewball32/xemu-cartographer/internal/websocket"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	discordbot "github.com/Stewball32/xemu-cartographer/internal/disgo"
	"github.com/Stewball32/xemu-cartographer/internal/disgo/commands"
	pb "github.com/Stewball32/xemu-cartographer/internal/pocketbase"
	_ "github.com/Stewball32/xemu-cartographer/internal/scraper/haloce"     // self-registering Halo: CE GameReader
	_ "github.com/Stewball32/xemu-cartographer/internal/websocket/handlers" // self-registering WS handlers
	_ "github.com/Stewball32/xemu-cartographer/internal/websocket/rooms"    // self-registering WS room types
)

// configureOverlayTokens wires the M10 overlay-token signer + default lifetime
// from the environment. OVERLAY_TOKEN_SECRET is the HMAC secret; if unset, an
// ephemeral random secret is used (tokens won't survive a restart — fine for
// dev, set it in production). OVERLAY_TOKEN_TTL_HOURS overrides the long-lived
// default lifetime. Must run before routes register + the WS handler mounts,
// since both use the Default signer.
func configureOverlayTokens() {
	secret := []byte(os.Getenv("OVERLAY_TOKEN_SECRET"))
	if len(secret) == 0 {
		secret = make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			log.Printf("overlay tokens: failed to generate ephemeral secret: %v", err)
		}
		log.Printf("overlay tokens: OVERLAY_TOKEN_SECRET unset — using an ephemeral secret; minted tokens won't survive a restart. Set OVERLAY_TOKEN_SECRET in production.")
	}
	overlaytoken.Configure(secret)

	if h := os.Getenv("OVERLAY_TOKEN_TTL_HOURS"); h != "" {
		if hrs, err := strconv.Atoi(h); err == nil && hrs > 0 {
			overlaysroutes.SetDefaultTTL(time.Duration(hrs) * time.Hour)
		}
	}
}

func main() {
	app := pocketbase.New()

	var bot *discordbot.Bot
	var hub *ws.Hub
	var watcherCancel context.CancelFunc
	var scrMgr *scrapermgr.Manager
	// podMgr is set below only when CONTAINERS_ENABLED; the host-runner URL
	// resolver reads it at call time (through a getter) so it can be wired before
	// podman is constructed.
	var podMgr *podman.Manager

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

		// Register snapshot hooks AFTER seeding so the seeder's own writes don't
		// overwrite the snapshot mid-run.
		seed.RegisterContainerSnapshotHooks(app)

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

		// Scraper manager: always available. Holds a *Services pointer; broadcasts
		// safely no-op until svc.WS is populated below. The blank import of
		// internal/scraper/haloce above triggers haloce.init(), which registers
		// Halo: CE's title ID with scraper.Lookup so manager.Start() can detect it.
		scrMgr = scrapermgr.New(svc)
		svc.Scraper = scrMgr
		scraperroutes.SetManager(scrMgr)
		// M09: the /api/me/match resolver (in the top-level routes package)
		// reads live container rosters to route a player to their kiosk.
		routes.SetScraper(scrMgr)

		// Player-hosting (ADR-0003): the host-runner Registry owns the per-instance
		// state-aware runners and fans their observable stream to the admin WS room.
		// It's wired to both the admin arbitration endpoints (/api/admin/scraper/
		// {name}/host) and the player-scoped /api/play/* group. The Manager attaches
		// a runner + vncinput input pump per instance on Start when HOSTRUNNER_ENABLED
		// — resolving each container's websockify URL through the podman manager
		// (nil-safe: no URL → runner ticks + emits state but presses nothing).
		hostReg := hostrunner.NewRegistry(newHostRunnerSink(svc))
		scraperroutes.SetHostControl(hostReg)
		playroutes.SetScraper(scrMgr)
		playroutes.SetHostControl(hostReg)
		// The play map picker is sourced LIVE per instance from the scraper (never
		// a stock table) — the Manager satisfies playroutes.MapSource.
		playroutes.SetMapSource(scrMgr)
		scrMgr.SetHostRunner(
			hostReg,
			hostRunnerURLResolver(func() *podman.Manager { return podMgr }),
			envBool("HOSTRUNNER_ENABLED", false),
		)

		// Capture-policy loader: read the persisted (instance, class) rows
		// now so runners started immediately after this (auto-start via the
		// discovery watcher, manual /api/admin/scraper/start) inherit the
		// current snapshot. The hook bind keeps them in sync as operators
		// edit policies through the PB dashboard.
		//
		// pb: sink scheme must register BEFORE the initial reload — a
		// policy carrying "pb:game_events" loaded against an empty registry
		// would error with "unknown scheme" and silently drop captures.
		sinks.RegisterPBSink(app)
		scrMgr.RegisterCapturePolicyHooks()
		if err := scrMgr.ReloadCapturePolicies(); err != nil {
			log.Printf("scraper: initial capture-policy load: %v", err)
		}

		// Containers (optional): start podman manager + socket watcher when
		// CONTAINERS_ENABLED=true. The route group registers itself as a
		// no-op when Manager is nil, so a fresh checkout boots cleanly.
		podmanCfg := podman.LoadFromEnv()
		if podmanCfg.Enabled {
			containersStore := resolvers.NewContainersStore(app)
			mgr, err := podman.NewManager(podmanCfg, containersStore)
			if err != nil {
				return err
			}
			podMgr = mgr // host-runner URL resolver reads this to find websockify ports
			containers.SetManager(mgr)
			containers.SetServices(svc)

			if podmanCfg.SocketDir != "" {
				ctx, cancel := context.WithCancel(context.Background())
				watcherCancel = cancel

				// Per-name dedup of repeated identical auto-start errors. After
				// M5 stage 5a, scraper.Manager.Start no longer rejects unknown
				// titles — that path is handled inside the runner's Idle phase.
				// Start only errors here on QMP init failure (xemu container
				// still booting / socket not yet ready), which is also a state
				// that resolves on its own. Dedup keeps the log clean during
				// the boot retry window.
				var (
					lastErrMu sync.Mutex
					lastErr   = map[string]string{}
				)
				var w *discovery.Watcher
				w = discovery.NewWatcher(podmanCfg.SocketDir, 2*time.Second,
					func(name, sock string) {
						go func() {
							err := scrMgr.Start(name, sock)
							if err == nil {
								lastErrMu.Lock()
								delete(lastErr, name)
								lastErrMu.Unlock()
								return
							}
							msg := err.Error()
							lastErrMu.Lock()
							prev := lastErr[name]
							lastErr[name] = msg
							lastErrMu.Unlock()
							if prev != msg {
								log.Printf("discovery: auto-start scraper %s: %v", name, err)
							}
							// Drop from the watcher's known set so the next poll
							// retries — typical case is xemu still booting / on the
							// dashboard, which resolves once a game is loaded.
							w.Forget(name)
						}()
					},
					func(name string) {
						lastErrMu.Lock()
						delete(lastErr, name)
						lastErrMu.Unlock()
						if err := scrMgr.Stop(name); err != nil {
							log.Printf("discovery: auto-stop scraper %s: %v", name, err)
						}
					},
				)
				go w.Run(ctx)
			}
		}

		configureOverlayTokens()
		routes.RegisterAll(se)

		hub = ws.NewHub(app)
		go hub.Run()
		ws.SetInstance(hub)
		se.Router.GET("/api/ws", ws.NewHandler(hub, app, scrMgr.SendHelloOn))
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
		// tick goroutine exits. After all runners are gone, Close() stops the
		// host:all aggregator goroutine so the process can exit cleanly.
		if scrMgr != nil {
			for _, info := range scrMgr.List() {
				if err := scrMgr.Stop(info.Name); err != nil {
					log.Printf("scraper: stop %s on shutdown: %v", info.Name, err)
				}
			}
			scrMgr.Close()
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
