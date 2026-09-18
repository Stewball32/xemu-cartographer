package xemu

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"

	"github.com/xemu-cartographer/xc-scraper/daemon"
	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/pb/pbtest"
	"github.com/xemu-cartographer/xemu-cartographer/internal/leaguescraper"
	scraperroutes "github.com/xemu-cartographer/xemu-cartographer/internal/pocketbase/routes/scraper"
	"github.com/xemu-cartographer/xemu-cartographer/internal/xcclient"
)

// TestParityRealDaemon is the DESIGN-STEP8 §16 checklist run that needs no
// xemu: a real xc/daemon (AllowEmpty), the real stream client + Adapter, and
// the scraper + xemu route groups mounted together. It pins the wire-mode
// contract end to end: 202 attaching, 409 / 400 from the daemon's codes,
// 404 for unattached instances, probe tools relayed with the daemon's
// error bodies, and 503 on every proxied route once the daemon is gone.
func TestParityRealDaemon(t *testing.T) {
	const feed, control = "feed-secret", "ctl-secret"
	cfg := daemon.Config{
		Listen: "127.0.0.1:0", Token: feed, ControlToken: control,
		AllowEmpty: true, StateDir: t.TempDir(), OffsetsDir: t.TempDir(), Version: "parity-test",
	}
	s, err := daemon.New(cfg)
	if err != nil {
		t.Fatalf("daemon.New: %v", err)
	}
	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		t.Fatal(err)
	}
	dctx, stopDaemon := context.WithCancel(context.Background())
	daemonDone := make(chan struct{})
	go func() { defer close(daemonDone); _ = s.Serve(dctx, ln) }()
	t.Cleanup(func() {
		stopDaemon()
		select {
		case <-daemonDone:
		case <-time.After(10 * time.Second):
			t.Error("daemon did not stop")
		}
	})

	c, err := xcclient.New(xcclient.Config{URL: "http://" + ln.Addr().String(), Token: feed, Log: t.Logf})
	if err != nil {
		t.Fatalf("xcclient.New: %v", err)
	}
	cctx, stopClient := context.WithCancel(context.Background())
	t.Cleanup(stopClient)
	go func() { _ = c.Run(cctx) }()
	ctl := xcclient.NewCtl(c, control)
	waitFor(t, "stream connected", func() bool { return ctl.Connected() })
	adapter := leaguescraper.NewAdapter(c, ctl)

	// Mount both groups on one router with the Adapter as every source.
	app, _ := pbtest.NewApp(t)
	su := pbtest.NewSuperuser(t, app, "admin@example.com")
	tok, err := su.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	scraperroutes.SetManager(adapter)
	scraperroutes.SetHostControl(adapter)
	scraperroutes.SetMapSource(adapter)
	t.Cleanup(func() {
		scraperroutes.Manager, scraperroutes.HostRunners, scraperroutes.Maps = nil, nil, nil
	})
	r, err := apis.NewRouter(app)
	if err != nil {
		t.Fatal(err)
	}
	se := &core.ServeEvent{App: app, Router: r}
	scraperroutes.RegisterAll(se)
	RegisterAll(se)
	mux, err := r.BuildMux()
	if err != nil {
		t.Fatal(err)
	}
	do := func(method, path, body string) (int, map[string]any) {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Authorization", tok)
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		_, out := decodeRec(rec)
		return rec.Code, out
	}

	sock := filepath.Join(t.TempDir(), "nox.sock") // never created: attach loop retries
	start := `{"name":"nox","sock":"` + sock + `"}`

	// Admin debug tabs row: start is asynchronous (202), then the daemon holds the name.
	code, body := do(http.MethodPost, "/api/admin/scraper/start", start)
	if code != http.StatusAccepted || body["phase"] != scraperroutes.PhaseAttaching || body["sock"] != sock {
		t.Fatalf("start: %d %v", code, body)
	}
	if code, _ := do(http.MethodPost, "/api/admin/scraper/start", start); code != http.StatusConflict {
		t.Errorf("second start: %d, want 409", code)
	}
	// Pod list row (§6.1 / §16): the list is the daemon's read-API view, so
	// the retrying attach is visible as phase "attaching" with its sock and
	// last error — the stream mirror alone would never show it.
	listRows := func() []xcclient.InstanceRow {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/admin/scraper", nil)
		req.Header.Set("Authorization", tok)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
		}
		var rows []xcclient.InstanceRow
		if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
			t.Fatalf("list decode: %v", err)
		}
		return rows
	}
	waitFor(t, "attach error recorded", func() bool { // 1 s cache; the loop's first failure lands right after the 202
		rows := listRows()
		return len(rows) == 1 && rows[0].Error != ""
	})
	if rows := listRows(); len(rows) != 1 || rows[0].Name != "nox" || rows[0].Phase != scraperroutes.PhaseAttaching || rows[0].Sock != sock || rows[0].Running {
		t.Errorf("list while attaching: %+v", rows)
	}
	// The daemon's bad_addr (relative unix path) maps onto ErrInvalidName → 400.
	if code, body := do(http.MethodPost, "/api/admin/scraper/start", `{"name":"nox","sock":"relative.sock"}`); code != http.StatusBadRequest {
		t.Errorf("relative sock: %d %v, want 400 (daemon bad_addr)", code, body)
	}
	// Not running yet: inspect / diagnostics are 404; GET host answers the
	// (absent) status like in-process and POST host is 404.
	for _, p := range []string{"/inspect", "/diagnostics"} {
		if code, _ := do(http.MethodGet, "/api/admin/scraper/nox"+p, ""); code != http.StatusNotFound {
			t.Errorf("GET nox%s: %d, want 404", p, code)
		}
	}
	if code, body := do(http.MethodGet, "/api/admin/scraper/nox/host", ""); code != http.StatusOK || body["present"] != false {
		t.Errorf("GET nox/host: %d %v, want 200 present=false", code, body)
	}
	if code, _ := do(http.MethodPost, "/api/admin/scraper/nox/host", `{"authority":"admin"}`); code != http.StatusNotFound {
		t.Errorf("POST nox/host: %d, want 404", code)
	}
	// xemu probe routes row: same paths/auth; the daemon's error bodies relay verbatim.
	code, body = do(http.MethodGet, "/api/admin/xemu/probe?sock="+sock, "")
	if code != http.StatusBadRequest || daemonCode(body) != "bad_addr" {
		t.Errorf("probe attached-but-absent sock: %d %v, want 400 bad_addr", code, body)
	}
	code, body = do(http.MethodGet, "/api/admin/xemu/scan-string?q=x&sock="+filepath.Join(t.TempDir(), "other.sock"), "")
	if code != http.StatusNotFound || daemonCode(body) != "not_attached" {
		t.Errorf("scan-string unattached sock: %d %v, want 404 not_attached", code, body)
	}
	// Stop = detach; a second stop is idempotent.
	for i := 0; i < 2; i++ {
		if code, _ := do(http.MethodPost, "/api/admin/scraper/nox/stop", ""); code != http.StatusNoContent {
			t.Errorf("stop #%d: %d, want 204", i+1, code)
		}
	}
	// Stop cancelled the retrying loop (post-verification fix): the row
	// reads "detached" (Stop drops the 1 s cache) and an immediate re-start
	// is accepted, not 409.
	if rows := listRows(); len(rows) != 1 || rows[0].Phase != daemon.PhaseDetached || rows[0].Sock != sock {
		t.Errorf("list after stop: %+v", rows)
	}
	if code, body := do(http.MethodPost, "/api/admin/scraper/start", start); code != http.StatusAccepted {
		t.Errorf("re-start after stop: %d %v, want 202", code, body)
	}
	if code, _ := do(http.MethodPost, "/api/admin/scraper/nox/stop", ""); code != http.StatusNoContent {
		t.Errorf("stop relaunched loop: %d, want 204", code)
	}

	// 503 while daemon down (Play picker / Admin rows): kill the daemon, wait
	// for the stream gate to drop, and every proxied route must fail fast.
	stopDaemon()
	waitFor(t, "stream disconnected", func() bool { return !ctl.Connected() })
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/api/admin/scraper/start"},
		{http.MethodGet, "/api/admin/scraper/nox/inspect"},
		{http.MethodGet, "/api/admin/scraper/nox/diagnostics"},
		{http.MethodGet, "/api/admin/scraper/nox/host"},
		{http.MethodPost, "/api/admin/scraper/nox/stop"},
		{http.MethodGet, "/api/admin/xemu/probe?sock=" + sock},
		{http.MethodGet, "/api/admin/xemu/probe-title?sock=" + sock},
		{http.MethodGet, "/api/admin/xemu/sample-deltas?sock=" + sock},
		{http.MethodGet, "/api/admin/xemu/scan-string?q=x&sock=" + sock},
	} {
		b := ""
		if tc.path == "/api/admin/scraper/start" {
			b = start
		}
		code, body := do(tc.method, tc.path, b)
		if code != http.StatusServiceUnavailable || body["error"] != scraperroutes.UnavailableMessage {
			t.Errorf("%s %s while down: %d %v, want 503", tc.method, tc.path, code, body)
		}
	}
	// The list 503s too (§11 / §12); /upstream keeps answering with the
	// disconnect timestamp — that is its point.
	if code, body := do(http.MethodGet, "/api/admin/scraper", ""); code != http.StatusServiceUnavailable || body["error"] != scraperroutes.UnavailableMessage {
		t.Errorf("list while down: %d %v, want 503", code, body)
	}
	if code, body := do(http.MethodGet, "/api/admin/scraper/upstream", ""); code != http.StatusOK || body["mode"] != scraperroutes.ModeWire || body["connected"] != false {
		t.Errorf("upstream while down: %d %v, want 200 mode=wire connected=false", code, body)
	}
}

func daemonCode(body map[string]any) string {
	e, _ := body["error"].(map[string]any)
	c, _ := e["code"].(string)
	return c
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}
