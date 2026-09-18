package leaguescraper

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/xemu-cartographer/xemu-cartographer/internal/podman"
	"github.com/xemu-cartographer/xemu-cartographer/internal/xcclient"
)

// Mode is the R1 scraper-feed mode (DESIGN-STEP8 D-4): XC_SCRAPER_URL set ⇒
// ModeWire (the league consumes the xc-scraper daemon), unset ⇒ ModeInProcess
// (today's embedded runner). R2 deletes the in-process path.
type Mode string

const (
	ModeWire      Mode = "wire"
	ModeInProcess Mode = "in-process"
)

// EnvUpstreamURL is the league-side switch (§11): `http(s)://host[:port]`.
const EnvUpstreamURL = "XC_SCRAPER_URL"

// BootConfig is the wire-mode boot input, read from the XC_SCRAPER_* env by
// cmd/server (§11). Every field but URL is optional.
type BootConfig struct {
	// URL is XC_SCRAPER_URL; ValidateUpstreamURL decides what is accepted.
	URL string
	// Token is the daemon feed token (XC_SCRAPER_TOKEN, "" when open).
	Token string
	// ControlToken is XC_SCRAPER_CONTROL_TOKEN for /api/ctl/*.
	ControlToken string
	// StaleAfter is XC_SCRAPER_STALE_AFTER (D-12; 0 = xcclient.DefaultStaleAfter).
	StaleAfter time.Duration
	// Hostrunner joins xc:hostrunner on connect (HOSTRUNNER_ENABLED, D-9).
	Hostrunner bool
	// HostDriveMarker is HOSTRUNNER_DRIVE_MARKER, pushed per instance (§10).
	HostDriveMarker string
	// Hub receives rebroadcasts + evictions (*ws.Hub); nil drops both.
	Hub xcclient.HubPort
	// Pods late-binds the podman manager for the config push (nil, or a
	// getter answering nil, means CONTAINERS_ENABLED is off).
	Pods func() PodSource
	// Logf receives one-line diagnostics; nil means log.Printf.
	Logf func(format string, args ...any)
}

// Wire is the assembled wire-mode consumer: the stream client + mirror, the
// control client, the Adapter every scraperiface consumer sees, the demand
// observer for the league hub, the rebroadcaster, the config pusher and the
// pb: event writer. Boot builds it idle; Start runs the stream.
type Wire struct {
	Client      *xcclient.Client
	Ctl         *xcclient.Ctl
	Adapter     *Adapter
	Demand      *xcclient.Demand
	Rebroadcast *xcclient.Rebroadcaster
	Pusher      *ConfigPusher
	Events      *EventWriter
	// Finished is the D-7 webhook-misconfiguration detector (§7.2): a
	// previous_game uid that never lands in games logs one line after 30 s.
	Finished *xcclient.FinishedGameDetector

	logf   func(string, ...any)
	cancel context.CancelFunc
	done   chan struct{}
}

// Boot validates cfg.URL (§11) and assembles the wire-mode consumer over
// app: xcclient + Adapter, demand + rebroadcast over cfg.Hub, the config
// pusher (PB hooks registered, first push on connect) and the event writer
// fed by the capture_policies rows. Nothing touches the network until
// Start. The caller wires cfg.Hub's room observer to Demand.Observe before
// the hub runs and the podman hooks to Pusher.PodHooks.
func Boot(app core.App, cfg BootConfig) (*Wire, error) {
	if err := ValidateUpstreamURL(cfg.URL); err != nil {
		return nil, err
	}
	logf := cfg.Logf
	if logf == nil {
		logf = log.Printf
	}
	w := &Wire{logf: logf}
	client, err := xcclient.New(xcclient.Config{
		URL:        strings.TrimSpace(cfg.URL),
		Token:      cfg.Token,
		StaleAfter: cfg.StaleAfter,
		Hub:        cfg.Hub,
		OnFrame:    w.onFrame,
		Log:        logf,
		Hostrunner: cfg.Hostrunner,
	})
	if err != nil {
		return nil, fmt.Errorf("leaguescraper: %w", err)
	}
	w.Client = client
	w.Ctl = xcclient.NewCtl(client, cfg.ControlToken)
	w.Adapter = NewAdapter(client, w.Ctl)
	w.Rebroadcast = xcclient.NewRebroadcaster(cfg.Hub, logf)
	w.Demand = xcclient.NewDemand(client, xcclient.DefaultLinger)
	var pods PodSource
	if cfg.Pods != nil {
		pods = latePods{get: cfg.Pods}
	}
	w.Pusher = NewConfigPusher(PushOptions{
		App:             app,
		Ctl:             w.Ctl,
		Pods:            pods,
		HostDriveMarker: cfg.HostDriveMarker,
		Logf:            logf,
	})
	w.Events = NewEventWriter(app, EventWriterOptions{
		Demand:    client,
		Instances: client.Mirror().Instances,
		Logf:      logf,
	})
	w.Finished = xcclient.NewFinishedGameDetector(gameExists(app), logf)
	client.OnConnect(w.Pusher.OnConnect)
	w.Pusher.RegisterHooks(app)
	// Same provider as embedded mode, pointed at the writer: pb: rows stay
	// league-side, the rest rides the config push (§10).
	RegisterCapturePolicyHooks(app, w.Events)
	if err := ReloadCapturePolicies(app, w.Events); err != nil {
		logf("scraper: initial capture-policy load: %v", err)
	}
	return w, nil
}

// onFrame is the composed xcclient.Config.OnFrame: hello tracking for the
// adapter, byte-identical rebroadcast to the league rooms, pb: event rows.
func (w *Wire) onFrame(f xcclient.Frame) {
	w.Adapter.OnFrame(f)
	w.Rebroadcast.OnFrame(f)
	w.Events.OnFrame(f)
	w.Finished.OnFrame(f)
}

// gameExists is the D-7 detector's games lookup: the same game_uid match
// the ingest route's dedupe uses (internal/games.PersistFinishedGame).
func gameExists(app core.App) func(uid string) bool {
	return func(uid string) bool {
		_, err := app.FindFirstRecordByFilter("games", "game_uid = {:uid}", dbx.Params{"uid": uid})
		// Only a definite miss counts: a read error must not fake the D-7 line.
		return !errors.Is(err, sql.ErrNoRows)
	}
}

// Start runs the stream (reconnect loop) on its own goroutine until Close.
// A second Start is a no-op.
func (w *Wire) Start() {
	if w.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	w.done = make(chan struct{})
	go func() {
		defer close(w.done)
		if err := w.Client.Run(ctx); err != nil && ctx.Err() == nil {
			w.logf("leaguescraper: upstream stream stopped: %v", err)
		}
	}()
}

// closeWait bounds how long Close waits for the stream goroutine.
const closeWait = 5 * time.Second

// Close stops the stream, the demand timers and the config pusher. Safe
// before Start and more than once.
func (w *Wire) Close() {
	if w.cancel != nil {
		w.cancel()
		select {
		case <-w.done:
		case <-time.After(closeWait):
			w.logf("leaguescraper: upstream stream did not stop within %s", closeWait)
		}
		w.cancel = nil
	}
	w.Demand.Close()
	w.Pusher.Close()
}

// BootLine is the §12 boot line: `leaguescraper: mode=wire url=… token=set
// control=set` or `leaguescraper: mode=in-process (XC_SCRAPER_URL unset)`.
func BootLine(mode Mode, url, token, controlToken string) string {
	if mode != ModeWire {
		return fmt.Sprintf("leaguescraper: mode=%s (%s unset)", ModeInProcess, EnvUpstreamURL)
	}
	return fmt.Sprintf("leaguescraper: mode=%s url=%s token=%s control=%s",
		ModeWire, strings.TrimSpace(url), setOrUnset(token), setOrUnset(controlToken))
}

func setOrUnset(v string) string {
	if v == "" {
		return "unset"
	}
	return "set"
}

// latePods defers the PodSource lookup to call time so the pusher can be
// built before the podman manager exists (or never exists).
type latePods struct{ get func() PodSource }

func (l latePods) src() PodSource {
	if l.get == nil {
		return nil
	}
	return l.get()
}

func (l latePods) List() ([]podman.ContainerInfo, error) {
	if p := l.src(); p != nil {
		return p.List()
	}
	return nil, nil
}

func (l latePods) OverlayPath(name string) (string, bool) {
	if p := l.src(); p != nil {
		return p.OverlayPath(name)
	}
	return "", false
}
