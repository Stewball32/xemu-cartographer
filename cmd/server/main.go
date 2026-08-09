package main

import (
	"context"
	"crypto/rand"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/discovery"
	"github.com/Stewball32/xemu-cartographer/internal/guards"
	"github.com/Stewball32/xemu-cartographer/internal/hostrunner"
	"github.com/Stewball32/xemu-cartographer/internal/overlaytoken"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/hooks"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/migrateconf"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/oauth"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/resolvers"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/containers"
	overlaysroutes "github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/overlays"
	playroutes "github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/play"
	scraperroutes "github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/pocketbase/seed"
	"github.com/Stewball32/xemu-cartographer/internal/podman"
	"github.com/Stewball32/xemu-cartographer/internal/reaper"
	scrapermgr "github.com/Stewball32/xemu-cartographer/internal/scraper/manager"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/sinks"
	ws "github.com/Stewball32/xemu-cartographer/internal/websocket"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"

	discordbot "github.com/Stewball32/xemu-cartographer/internal/disgo"
	"github.com/Stewball32/xemu-cartographer/internal/disgo/commands"
	pb "github.com/Stewball32/xemu-cartographer/internal/pocketbase"
	_ "github.com/Stewball32/xemu-cartographer/internal/scraper/halo2"      // self-registering Halo 2 GameReader (M20)
	_ "github.com/Stewball32/xemu-cartographer/internal/scraper/haloce"     // self-registering Halo: CE GameReader
	_ "github.com/Stewball32/xemu-cartographer/internal/websocket/handlers" // self-registering WS handlers
	_ "github.com/Stewball32/xemu-cartographer/internal/websocket/rooms"    // self-registering WS room types
	_ "github.com/Stewball32/xemu-cartographer/migrations"                  // self-registering DB migrations (schema source of truth)
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
		// FAIL-OPEN default (dev convenience): with no configured secret, tokens
		// are signed with a random secret regenerated every boot, so every minted
		// overlay token silently becomes invalid after a restart (a quiet prod
		// outage). Behavior is intentionally unchanged; this only warns loudly.
		// Recommendation: set a stable OVERLAY_TOKEN_SECRET in production (and
		// consider failing closed there — see docs/review-fixes-2026-07-12.md).
		log.Printf("SECURITY WARNING: OVERLAY_TOKEN_SECRET unset — signing overlay tokens with an EPHEMERAL secret; every minted token is invalidated on each restart. Set a stable OVERLAY_TOKEN_SECRET in production.")
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
	var reaperCancel context.CancelFunc
	var scrMgr *scrapermgr.Manager
	// podMgr is set below only when CONTAINERS_ENABLED; the host-runner URL
	// resolver reads it at call time (through a getter) so it can be wired before
	// podman is constructed.
	var podMgr *podman.Manager

	// Database migrations are the SOURCE OF TRUTH for schema (docs/MIGRATIONS.md).
	// Pending migrations in migrations/ are applied automatically on boot — BEFORE
	// OnServe — and tracked in the _migrations table, so routes/hooks/the scraper
	// can assume the schema exists. Automigrate writes a migration file whenever
	// the schema changes in the admin UI, but ONLY in dev builds (`-tags dev`);
	// beta/prod snapshots apply reviewed migrations and never author new ones.
	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		Dir:          "migrations",
		Automigrate:  migrateconf.Automigrate,
		TemplateLang: migratecmd.TemplateLangGo,
	})

	// Register record lifecycle hooks (callback registration, fires later).
	hooks.RegisterAll(app)

	// OnServe: register routes (needs running DB / ServeEvent).
	//
	// NOTE: collections are created by MIGRATIONS (migrations/), applied on boot
	// BEFORE this hook — there is no schema-as-code step any more (the retired
	// internal/pocketbase/schema package). Anything here may assume the schema
	// exists. See docs/MIGRATIONS.md.
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		if err := oauth.RegisterAll(app); err != nil {
			return err
		}

		if err := seed.Run(app); err != nil {
			return err
		}

		// Env-driven superuser bootstrap (all builds, incl. prod-style beta) —
		// creates a superuser from SEED_SUPERUSER_EMAIL/PASSWORD if set and none
		// with that email exists yet. No-op when unset.
		if err := seed.EnsureEnvSuperuser(app); err != nil {
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
		// The admin diagnostics panel shows the same enumerated carousel, and reads the
		// LIVE per-tick readout (not the host runner's last event) so it stays live on
		// an observed-only box.
		scraperroutes.SetMapSource(scrMgr)
		scraperroutes.SetReadoutSource(scrMgr)
		scrMgr.SetHostRunner(
			hostReg,
			hostRunnerURLResolver(func() *podman.Manager { return podMgr }),
			envBool("HOSTRUNNER_ENABLED", false),
		)
		// Host/client scoping (pod-hijack fix): AUTO-DRIVE only player-hosted
		// boxes — the ones /api/play/request provisions as "<prefix>play-<uid>"
		// (the same "play-" marker the reaper scopes idle-out to). Every other
		// box (admin/manual — e.g. a client pod created to JOIN a System Link
		// lobby) attaches observe-only and is never driven until an admin
		// promotes it via the host control endpoint. Marker is env-tunable for
		// non-standard deployments.
		driveMarker := envStr("HOSTRUNNER_DRIVE_MARKER", "play-")
		scrMgr.SetHostDrivePolicy(func(name string) bool {
			return strings.Contains(name, driveMarker)
		})
		// Host-side custom gametype variant enumeration (part C): resolve a box's
		// overlay qcow2 so the runner can read its saved variants off disk (the
		// SELECT GAMETYPE carousel keeps only rendered cards resident). nil-safe —
		// no podman manager (CONTAINERS_ENABLED off) → built-in gametypes only.
		scrMgr.SetOverlayResolver(func(name string) (string, bool) {
			if podMgr == nil {
				return "", false
			}
			return podMgr.OverlayPath(name)
		})

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

			// Offset-set selection (offset versioning): map an instance to the
			// offset-set id its catalog row assigns. Under the managed ingest
			// model the attached disc is <isos-record-id>.iso, so the podman
			// GameISO basename IS the record id — no extra linkage. Empty (or
			// any lookup miss) means the detected game's baseline; boxes whose
			// GameISO is unknown (e.g. attached before a server restart) also
			// fall back to the baseline. Fail-soft by design.
			scrMgr.SetOffsetSetResolver(func(instance string) string {
				info, ok := mgr.Get(instance)
				if !ok || info.GameISO == "" {
					return ""
				}
				id := strings.TrimSuffix(filepath.Base(info.GameISO), ".iso")
				rec, err := app.FindRecordById("isos", id)
				if err != nil {
					return ""
				}
				return rec.GetString("offset_set")
			})
			containers.SetManager(mgr)
			containers.SetServices(svc)
			// Player request-instance flow: provisions a fresh box booting the
			// chosen catalog ISO. Additive to the untouched admin kiosk/VNC path.
			playroutes.SetProvisioner(podmanProvisioner{m: mgr})

			// Idle-out reaper (optional, REAPER_ENABLED): reclaim player-hosted
			// boxes that nobody joins — an empty lobby with no live match and no
			// guest machine for the idle window is torn down (stop scraper +
			// podman remove). Scoped to the "play-" prefix so admin/manual boxes
			// are never auto-reaped. Reads activity from the scraper + host-runner;
			// removes through this podman manager. A disabled config makes Run a
			// no-op, so the wiring is unconditional and just doesn't spin a
			// goroutine.
			reap := reaper.New(
				reaperConfigFromEnv(),
				reaperSource{scr: scrMgr, host: hostReg},
				reaperRemover{scr: scrMgr, pod: mgr},
				reaper.WithLogger(log.Printf),
			)
			if reap.Enabled() {
				// Surface the per-instance countdown on /api/play/current so the
				// host gets a heads-up before an idle box is reclaimed.
				playroutes.SetIdleReporter(reaperIdleReporter{r: reap})
				rctx, rcancel := context.WithCancel(context.Background())
				reaperCancel = rcancel
				go reap.Run(rctx)
				log.Printf("reaper: idle-out enabled (REAPER_* env controls timeout/prefix)")
			}

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
		// Stop the reaper before the scrapers so no reap pass fires against a
		// manager that's tearing down.
		if reaperCancel != nil {
			reaperCancel()
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
