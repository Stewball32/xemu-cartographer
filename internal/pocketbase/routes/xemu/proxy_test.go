package xemu

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/authz/pb/pbtest"
	"github.com/Stewball32/xemu-cartographer/internal/leaguescraper"
	scraperroutes "github.com/Stewball32/xemu-cartographer/internal/pocketbase/routes/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/xcclient"
)

const testControlToken = "ctl-secret"

// xemuDaemon stubs POST /api/ctl/xemu/{tool}: control-token gated, 404
// not_attached unless the addr names smoke1, otherwise it echoes what it
// received so the test can assert the query → body re-encoding.
type xemuDaemon struct {
	srv   *httptest.Server
	mu    sync.Mutex
	last  map[string]any
	delay time.Duration // how long each tool takes to answer
}

func newXemuDaemon(t *testing.T) *xemuDaemon {
	t.Helper()
	d := &xemuDaemon{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/ctl/xemu/{tool}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("Authorization") != "Bearer "+testControlToken {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"code":"unauthorized","message":"bad control token"}}`))
			return
		}
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		d.mu.Lock()
		d.last = body
		delay := d.delay
		d.mu.Unlock()
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
			return
		}
		addr, _ := body["addr"].(string)
		if !strings.HasSuffix(addr, "/smoke1.sock") {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "not_attached", "message": addr + " is not an attached instance"}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"tool": r.PathValue("tool"), "addr": addr, "echo": body})
	})
	d.srv = httptest.NewServer(mux)
	t.Cleanup(d.srv.Close)
	return d
}

// mount registers the xemu group on a real router with src as the scraper
// route source (which is what wireCtl reads) and returns mux + admin token.
func mount(t *testing.T, src any) (http.Handler, string) {
	t.Helper()
	app, _ := pbtest.NewApp(t)
	su := pbtest.NewSuperuser(t, app, "admin@example.com")
	tok, err := su.NewAuthToken()
	if err != nil {
		t.Fatalf("NewAuthToken: %v", err)
	}
	switch s := src.(type) {
	case *leaguescraper.Adapter:
		scraperroutes.SetManager(s)
	case *pbtest.FakeScraper:
		scraperroutes.SetManager(s)
	default:
		scraperroutes.Manager = nil
	}
	t.Cleanup(func() { scraperroutes.Manager = nil })
	r, err := apis.NewRouter(app)
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	RegisterAll(&core.ServeEvent{App: app, Router: r})
	mux, err := r.BuildMux()
	if err != nil {
		t.Fatalf("BuildMux: %v", err)
	}
	return mux, tok
}

func get(t *testing.T, mux http.Handler, tok, path string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", tok)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return decodeRec(rec)
}

// decodeRec returns the recorded status + JSON body (nil when empty).
func decodeRec(rec *httptest.ResponseRecorder) (int, map[string]any) {
	var out map[string]any
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec.Code, out
}

func newWire(t *testing.T) (*xemuDaemon, *leaguescraper.Adapter, *xcclient.Ctl) {
	t.Helper()
	d := newXemuDaemon(t)
	c, err := xcclient.New(xcclient.Config{URL: d.srv.URL, Token: "feed-secret"})
	if err != nil {
		t.Fatalf("xcclient.New: %v", err)
	}
	ctl := xcclient.NewCtl(c, testControlToken)
	ctl.Connected = func() bool { return true }
	return d, leaguescraper.NewAdapter(c, ctl), ctl
}

// TestProxyProbeWire: ?sock= becomes the daemon's unix: addr, the daemon's
// body + status come back verbatim (200 here, 404 not_attached for a socket
// the daemon does not own).
func TestProxyProbeWire(t *testing.T) {
	d, adapter, _ := newWire(t)
	mux, tok := mount(t, adapter)

	code, body := get(t, mux, tok, "/api/admin/xemu/probe?sock=/run/smoke1.sock")
	if code != http.StatusOK || body["tool"] != "probe" || body["addr"] != "unix:/run/smoke1.sock" {
		t.Fatalf("probe: %d %v", code, body)
	}
	code, body = get(t, mux, tok, "/api/admin/xemu/probe?sock=unix:/run/smoke1.sock")
	if code != http.StatusOK || body["addr"] != "unix:/run/smoke1.sock" {
		t.Fatalf("probe qualified addr: %d %v", code, body)
	}
	code, body = get(t, mux, tok, "/api/admin/xemu/probe?sock=/run/other.sock")
	if code != http.StatusNotFound {
		t.Fatalf("probe unattached: %d %v, want 404 relayed", code, body)
	}
	if errObj, _ := body["error"].(map[string]any); errObj["code"] != "not_attached" {
		t.Errorf("daemon error body not relayed verbatim: %v", body)
	}
	d.mu.Lock()
	last := d.last
	d.mu.Unlock()
	if last["addr"] != "unix:/run/other.sock" {
		t.Errorf("daemon saw %v", last)
	}
}

// TestProxyKnobsWire: every query knob of the four tools is re-encoded into
// the JSON body with the daemon's types (hex ranges as strings, counts as
// ints); absent knobs are left out so the daemon applies its own defaults.
func TestProxyKnobsWire(t *testing.T) {
	d, adapter, _ := newWire(t)
	mux, tok := mount(t, adapter)
	want := map[string]map[string]any{
		"/api/admin/xemu/scan-string?sock=/run/smoke1.sock&q=NICKNAME&start=0x81000000&end=0x90000000&encoding=utf16le&max=5&context=16": {
			"addr": "unix:/run/smoke1.sock", "q": "NICKNAME", "start": "0x81000000", "end": "0x90000000",
			"encoding": "utf16le", "max": float64(5), "context": float64(16),
		},
		"/api/admin/xemu/sample-deltas?sock=/run/smoke1.sock&start=0x100&end=0x200&interval_ms=250&max=3": {
			"addr": "unix:/run/smoke1.sock", "start": "0x100", "end": "0x200", "interval_ms": float64(250), "max": float64(3),
		},
		"/api/admin/xemu/probe-title?sock=/run/smoke1.sock&samples=4&interval_ms=abc": {
			"addr": "unix:/run/smoke1.sock", "samples": float64(4),
		},
	}
	for path, exp := range want {
		code, body := get(t, mux, tok, path)
		if code != http.StatusOK {
			t.Errorf("%s: %d %v", path, code, body)
			continue
		}
		d.mu.Lock()
		got := d.last
		d.mu.Unlock()
		if len(got) != len(exp) {
			t.Errorf("%s: body %v, want %v", path, got, exp)
		}
		for k, v := range exp {
			if got[k] != v {
				t.Errorf("%s: %s = %v (%T), want %v", path, k, got[k], got[k], v)
			}
		}
	}
	if code, _ := get(t, mux, tok, "/api/admin/xemu/scan-string?sock=/run/smoke1.sock"); code != http.StatusBadRequest {
		t.Errorf("scan-string without q: %d, want 400 (validated locally)", code)
	}
}

// TestProxyUpstreamDown503 + in-process fall-through: with the stream away
// every tool is 503 before any daemon call; without a wire source the
// handlers keep validating the local socket path (400 when it is missing).
func TestProxyUpstreamDownAndInProcess(t *testing.T) {
	d, adapter, ctl := newWire(t)
	mux, tok := mount(t, adapter)
	ctl.Connected = func() bool { return false }
	for _, path := range []string{"/probe", "/probe-title", "/sample-deltas", "/scan-string?q=x"} {
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		code, body := get(t, mux, tok, "/api/admin/xemu"+path+sep+"sock=/run/smoke1.sock")
		if code != http.StatusServiceUnavailable || body["error"] != scraperroutes.UnavailableMessage {
			t.Errorf("%s while down: %d %v", path, code, body)
		}
	}
	d.mu.Lock()
	called := d.last != nil
	d.mu.Unlock()
	if called {
		t.Error("daemon was called while down")
	}

	mux, tok = mount(t, &pbtest.FakeScraper{})
	code, body := get(t, mux, tok, "/api/admin/xemu/probe?sock=/nonexistent/smoke1.sock")
	if code != http.StatusBadRequest || !strings.Contains(body["error"].(string), "not accessible") {
		t.Fatalf("in-process probe: %d %v, want 400 not accessible", code, body)
	}
}

// TestProxyOutlivesCtlTimeout: a probe relay runs on its own budget — a
// daemon-side sampling longer than the 2 s control timeout still answers 200
// instead of a 502 context deadline, and the knobs widen the budget.
func TestProxyOutlivesCtlTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("sleeps past DefaultCtlTimeout")
	}
	d, adapter, ctl := newWire(t)
	mux, tok := mount(t, adapter)
	d.mu.Lock()
	d.delay = xcclient.DefaultCtlTimeout + 500*time.Millisecond
	d.mu.Unlock()

	code, body := get(t, mux, tok, "/api/admin/xemu/probe-title?sock=/run/smoke1.sock&samples=100&interval_ms=50")
	if code != http.StatusOK || body["tool"] != "probe_title" {
		t.Fatalf("probe_title past the control timeout: %d %v, want 200", code, body)
	}
	if ctl.Timeout != 0 {
		t.Fatalf("shared Ctl timeout mutated to %s", ctl.Timeout)
	}

	cases := []struct {
		body map[string]any
		want time.Duration
	}{
		{map[string]any{"addr": "unix:/x"}, probeBudgetBase},
		{map[string]any{"samples": 100, "interval_ms": 50}, probeBudgetBase + 5*time.Second},
		{map[string]any{"interval_ms": 1000}, probeBudgetBase + time.Second},
		{map[string]any{"samples": 1 << 20, "interval_ms": 1 << 20}, probeBudgetBase + 6000*time.Second},
		{map[string]any{"samples": -3, "interval_ms": -1}, probeBudgetBase},
	}
	for _, tc := range cases {
		if got := probeBudget(tc.body); got != tc.want {
			t.Errorf("probeBudget(%v) = %s, want %s", tc.body, got, tc.want)
		}
	}
}
