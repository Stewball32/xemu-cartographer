package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/xemu-cartographer/xemu-cartographer/internal/guards"
	"github.com/xemu-cartographer/xemu-cartographer/internal/leaguescraper"
	ws "github.com/xemu-cartographer/xemu-cartographer/internal/websocket"
)

// R1 dual-mode boot (DESIGN-STEP8 D-4, §11): XC_SCRAPER_URL set ⇒ wire mode
// (no runner.Manager, no discovery, the leaguescraper Adapter on the
// service surface); unset ⇒ the in-process manager exactly as before.

// newFeedApp bootstraps a bare PocketBase app (system migrations only). The
// package-main test binary links the flagship `migrations` package, and
// tests.NewTestApp() would replay its collections snapshot, which the Go
// jsonv2 experiment cannot decode (core.Collection.UnmarshalJSON recursion,
// see DESIGN-STEP8 §19). The feed boot only binds hooks and logs the
// missing capture_policies collection, so the bare app is enough.
func newFeedApp(t *testing.T) core.App {
	t.Helper()
	app := core.NewBaseApp(core.BaseAppConfig{DataDir: t.TempDir()})
	if err := app.Bootstrap(); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	t.Cleanup(func() { _ = app.ResetBootstrapState() })
	return app
}

func envOf(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func bootFeed(t *testing.T, env map[string]string) (*scraperFeed, *guards.Services) {
	t.Helper()
	app := newFeedApp(t)
	svc := &guards.Services{App: app}
	hub := ws.NewHub(app)
	feed, err := bootScraperFeed(app, svc, hub, envOf(env), nil)
	if err != nil {
		t.Fatalf("bootScraperFeed: %v", err)
	}
	t.Cleanup(feed.stop)
	return feed, svc
}

func TestBootScraperFeedInProcessWhenURLUnset(t *testing.T) {
	feed, _ := bootFeed(t, map[string]string{})
	if feed.mode != leaguescraper.ModeInProcess {
		t.Fatalf("mode = %q, want in-process", feed.mode)
	}
	if feed.mgr == nil || feed.wire != nil {
		t.Fatalf("in-process feed: mgr=%v wire=%v", feed.mgr != nil, feed.wire != nil)
	}
	if _, ok := feed.adapter.(*leaguescraper.WireAdapter); !ok {
		t.Fatalf("adapter = %T, want *leaguescraper.WireAdapter", feed.adapter)
	}
	if feed.hello == nil {
		t.Fatal("hello hook missing")
	}
	if want := "leaguescraper: mode=in-process (XC_SCRAPER_URL unset)"; feed.line != want {
		t.Fatalf("boot line = %q, want %q", feed.line, want)
	}
	if !feed.wantsDiscovery("/tmp/qmp") {
		t.Fatal("in-process mode with a socket dir must run discovery")
	}
	if feed.wantsDiscovery("") {
		t.Fatal("no socket dir ⇒ no discovery")
	}
	if _, ok := feed.reaperSource(nil).(reaperSource); !ok {
		t.Fatalf("reaper source = %T, want reaperSource", feed.reaperSource(nil))
	}
	if _, ok := feed.reaperRemover(nil).(reaperRemover); !ok {
		t.Fatalf("reaper remover = %T, want reaperRemover", feed.reaperRemover(nil))
	}
}

func TestBootScraperFeedWireWhenURLSet(t *testing.T) {
	// 127.0.0.1:1 is never listening: the client must boot regardless and
	// just keep reconnecting (daemon down at league boot, §12).
	feed, _ := bootFeed(t, map[string]string{
		"XC_SCRAPER_URL":           " http://127.0.0.1:1 ",
		"XC_SCRAPER_TOKEN":         "feed-secret",
		"XC_SCRAPER_CONTROL_TOKEN": "ctl-secret",
		"XC_SCRAPER_STALE_AFTER":   "90s",
		"HOSTRUNNER_ENABLED":       "true",
		"CONTAINERS_SOCKET_DIR":    "/tmp/qmp",
	})
	if feed.mode != leaguescraper.ModeWire {
		t.Fatalf("mode = %q, want wire", feed.mode)
	}
	if feed.mgr != nil {
		t.Fatal("wire mode must never build a runner.Manager")
	}
	if feed.wire == nil || feed.wire.Client == nil || feed.wire.Adapter == nil {
		t.Fatal("wire feed incomplete")
	}
	if _, ok := feed.adapter.(*leaguescraper.Adapter); !ok {
		t.Fatalf("adapter = %T, want *leaguescraper.Adapter", feed.adapter)
	}
	if feed.hello == nil {
		t.Fatal("hello hook missing")
	}
	if want := "leaguescraper: mode=wire url=http://127.0.0.1:1 token=set control=set"; feed.line != want {
		t.Fatalf("boot line = %q, want %q", feed.line, want)
	}
	if feed.wantsDiscovery("/tmp/qmp") {
		t.Fatal("wire mode must not run the league-side discovery watcher")
	}
	if !feed.hostrunner {
		t.Fatal("HOSTRUNNER_ENABLED not carried into the feed")
	}
	src, ok := feed.reaperSource(nil).(mirrorReaperSource)
	if !ok {
		t.Fatalf("reaper source = %T, want mirrorReaperSource", feed.reaperSource(nil))
	}
	if src.host == nil {
		t.Fatal("hostrunner on ⇒ reaper polls the daemon /host route")
	}
	if _, ok := feed.reaperRemover(nil).(podmanReaperRemover); !ok {
		t.Fatalf("reaper remover = %T, want podmanReaperRemover", feed.reaperRemover(nil))
	}
	if got := feed.wire.Ctl.ControlToken; got != "ctl-secret" {
		t.Fatalf("control token = %q", got)
	}
	if got := len(feed.wire.Adapter.List()); got != 0 {
		t.Fatalf("empty mirror expected, got %d instances", got)
	}

	// Boots and shuts down cleanly while the upstream is unreachable.
	feed.start()
	feed.start() // idempotent
	time.Sleep(50 * time.Millisecond)
	st := feed.wire.Client.Status()
	if st.Connected {
		t.Fatal("nothing listens on 127.0.0.1:1, Connected must be false")
	}
	feed.stop()
	feed.stop() // idempotent
}

func TestBootScraperFeedWireTokensUnset(t *testing.T) {
	feed, _ := bootFeed(t, map[string]string{"XC_SCRAPER_URL": "https://scraper.example"})
	if want := "leaguescraper: mode=wire url=https://scraper.example token=unset control=unset"; feed.line != want {
		t.Fatalf("boot line = %q, want %q", feed.line, want)
	}
	if src := feed.reaperSource(nil).(mirrorReaperSource); src.host != nil {
		t.Fatal("hostrunner off ⇒ the reaper must not poll /host")
	}
}

func TestBootScraperFeedRejectsBadURL(t *testing.T) {
	app := newFeedApp(t)
	svc := &guards.Services{App: app}
	for _, raw := range []string{"ws://127.0.0.1:8990", "http://127.0.0.1:8990/api/ws", "127.0.0.1:8990"} {
		_, err := bootScraperFeed(app, svc, ws.NewHub(app), envOf(map[string]string{"XC_SCRAPER_URL": raw}), nil)
		if !errors.Is(err, leaguescraper.ErrBadUpstreamURL) {
			t.Fatalf("%q: err = %v, want ErrBadUpstreamURL", raw, err)
		}
		if !strings.Contains(err.Error(), "XC_SCRAPER_URL must be http(s)://host[:port]") {
			t.Fatalf("%q: message %q", raw, err)
		}
	}
	_, err := bootScraperFeed(app, svc, ws.NewHub(app), envOf(map[string]string{
		"XC_SCRAPER_URL":         "http://127.0.0.1:8990",
		"XC_SCRAPER_STALE_AFTER": "soon",
	}), nil)
	if err == nil || !strings.Contains(err.Error(), "XC_SCRAPER_STALE_AFTER") {
		t.Fatalf("bad stale-after: err = %v", err)
	}
}
