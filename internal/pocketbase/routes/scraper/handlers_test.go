package scraper

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/authz/pb/pbtest"
	"github.com/xemu-cartographer/xc-scraper/runner"
)

// TestWireHelpersAreSourceAgnostic pins the mode detection: only a source
// exposing Ctl() (the wire Adapter) is "wire", and UpstreamDown follows the
// Ctl's fail-fast gate; the in-process fake is never down.
func TestWireHelpersAreSourceAgnostic(t *testing.T) {
	if Wire(nil) != nil || Wire(&pbtest.FakeScraper{}) != nil || UpstreamDown(&pbtest.FakeScraper{}) {
		t.Fatal("in-process source must not look like wire mode")
	}
	f := newWireFixture(t)
	if Wire(f.adapter) != f.ctl {
		t.Fatal("Wire(adapter) must return the adapter's Ctl")
	}
	if UpstreamDown(f.adapter) {
		t.Fatal("connected fixture reported down")
	}
	f.setUp(false)
	if !UpstreamDown(f.adapter) {
		t.Fatal("disconnected fixture reported up")
	}
}

// TestStartInProcess keeps today's synchronous contract: 201 + Info, 409 /
// 400 on the runner sentinels, 502 on any other Start failure.
func TestStartInProcess(t *testing.T) {
	src := &inProcessScraper{FakeScraper: &pbtest.FakeScraper{}}
	mux, tok := newMux(t, src, stubHost{})

	code, body := call(t, mux, tok, http.MethodPost, "/api/admin/scraper/start", `{"name":"box1","sock":"/run/box1.sock"}`)
	if code != http.StatusCreated || body["name"] != "box1" {
		t.Fatalf("start: %d %v, want 201 + Info", code, body)
	}
	src.startErr = runner.ErrAlreadyRunning
	if code, _ := call(t, mux, tok, http.MethodPost, "/api/admin/scraper/start", `{"name":"box1","sock":"/run/box1.sock"}`); code != http.StatusConflict {
		t.Errorf("already running: %d, want 409", code)
	}
	src.startErr = runner.ErrInvalidName
	if code, _ := call(t, mux, tok, http.MethodPost, "/api/admin/scraper/start", `{"name":"box:1","sock":"/run/x.sock"}`); code != http.StatusBadRequest {
		t.Errorf("invalid name: %d, want 400", code)
	}
	src.startErr = errors.New("qmp: init failed")
	if code, _ := call(t, mux, tok, http.MethodPost, "/api/admin/scraper/start", `{"name":"box2","sock":"/run/box2.sock"}`); code != http.StatusBadGateway {
		t.Errorf("init failure: %d, want 502", code)
	}
	if code, _ := call(t, mux, tok, http.MethodPost, "/api/admin/scraper/start", `{"name":"box2"}`); code != http.StatusBadRequest {
		t.Errorf("missing sock: %d, want 400", code)
	}
	if code, _ := call(t, mux, tok, http.MethodPost, "/api/admin/scraper/box1/stop", ""); code != http.StatusNoContent {
		t.Errorf("stop: %d, want 204", code)
	}
	if code, _ := call(t, mux, tok, http.MethodGet, "/api/admin/scraper/box1/inspect", ""); code != http.StatusOK {
		t.Errorf("inspect: %d, want 200", code)
	}
	if code, _ := call(t, mux, tok, http.MethodGet, "/api/admin/scraper/nope/inspect", ""); code != http.StatusNotFound {
		t.Errorf("inspect unknown: %d, want 404", code)
	}
}

// TestStartWireAccepted is the §16 degradation: in wire mode POST /start is
// asynchronous — the daemon's 202 becomes a 202 {name, sock, phase:
// "attaching"} here, the sock travels as a unix: addr under the control
// token, and the daemon's 409 / 400 codes map onto today's statuses.
func TestStartWireAccepted(t *testing.T) {
	f := newWireFixture(t)
	mux, tok := newMux(t, f.adapter, f.adapter)

	code, body := call(t, mux, tok, http.MethodPost, "/api/admin/scraper/start", `{"name":"box1","sock":"/run/box1.sock"}`)
	if code != http.StatusAccepted || body["name"] != "box1" || body["sock"] != "/run/box1.sock" || body["phase"] != PhaseAttaching {
		t.Fatalf("start: %d %v, want 202 attaching", code, body)
	}
	path, sent, auth := f.daemon.last()
	if path != "/api/ctl/attach" || sent["name"] != "box1" || sent["addr"] != "unix:/run/box1.sock" || auth != "Bearer "+testControlToken {
		t.Fatalf("daemon saw %s %v auth=%q", path, sent, auth)
	}
	if code, _ := call(t, mux, tok, http.MethodPost, "/api/admin/scraper/start", `{"name":"dup","sock":"/run/dup.sock"}`); code != http.StatusConflict {
		t.Errorf("already_running: %d, want 409", code)
	}
	if code, _ := call(t, mux, tok, http.MethodPost, "/api/admin/scraper/start", `{"name":"bad name","sock":"/run/x.sock"}`); code != http.StatusBadRequest {
		t.Errorf("invalid_name: %d, want 400", code)
	}
	// Stop = DELETE …/instances/{n}; the daemon's 404 is an idempotent 204.
	if code, _ := call(t, mux, tok, http.MethodPost, "/api/admin/scraper/smoke1/stop", ""); code != http.StatusNoContent {
		t.Errorf("stop attached: %d, want 204", code)
	}
	if code, _ := call(t, mux, tok, http.MethodPost, "/api/admin/scraper/ghost/stop", ""); code != http.StatusNoContent {
		t.Errorf("stop unknown: %d, want 204", code)
	}
}

// TestWireReadProxies covers inspect / diagnostics / host over the read API:
// the composed daemon diagnostics keep the diagnosticsResponse shape
// (highlighted_* resolved from the cursors, enumerated_* from maps), and an
// unattached name is 404.
func TestWireReadProxies(t *testing.T) {
	f := newWireFixture(t)
	mux, tok := newMux(t, f.adapter, f.adapter)

	code, body := call(t, mux, tok, http.MethodGet, "/api/admin/scraper/smoke1/inspect", "")
	if code != http.StatusOK || body["name"] != "smoke1" || body["phase"] != "ready" {
		t.Fatalf("inspect: %d %v", code, body)
	}
	if code, _ := call(t, mux, tok, http.MethodGet, "/api/admin/scraper/ghost/inspect", ""); code != http.StatusNotFound {
		t.Errorf("inspect unknown: %d, want 404", code)
	}

	code, body = call(t, mux, tok, http.MethodGet, "/api/admin/scraper/smoke1/diagnostics", "")
	if code != http.StatusOK {
		t.Fatalf("diagnostics: %d %v", code, body)
	}
	if body["highlighted_map"] != "Blood Gulch" || body["highlighted_gametype"] != "Slayer" || body["present"] != true || body["authority"] != "runner" {
		t.Errorf("diagnostics composed wrong: %v", body)
	}
	if maps, _ := body["enumerated_maps"].([]any); len(maps) != 3 || maps[2] != "Chill Out" {
		t.Errorf("enumerated_maps = %v", body["enumerated_maps"])
	}
	if body["host_health"] == nil || body["host_health_age_ms"] != float64(250) {
		t.Errorf("host_health not relayed: %v %v", body["host_health"], body["host_health_age_ms"])
	}
	if code, body := call(t, mux, tok, http.MethodGet, "/api/admin/scraper/ghost/diagnostics", ""); code != http.StatusNotFound || !strings.Contains(body["error"].(string), "ghost") {
		t.Errorf("diagnostics unknown: %d %v, want 404", code, body)
	}

	code, body = call(t, mux, tok, http.MethodGet, "/api/admin/scraper/smoke1/host", "")
	if code != http.StatusOK || body["present"] != true || body["authority"] != "runner" {
		t.Fatalf("host: %d %v", code, body)
	}
	code, body = call(t, mux, tok, http.MethodPost, "/api/admin/scraper/smoke1/host", `{"authority":"admin"}`)
	if code != http.StatusOK || body["instance"] != "smoke1" {
		t.Fatalf("set authority: %d %v", code, body)
	}
	if path, sent, _ := f.daemon.last(); path != "/api/ctl/instances/smoke1/host" || sent["authority"] != "admin" {
		t.Errorf("daemon saw %s %v", path, sent)
	}
	if code, _ := call(t, mux, tok, http.MethodPost, "/api/admin/scraper/ghost/host", `{"authority":"admin"}`); code != http.StatusNotFound {
		t.Errorf("set authority unknown: %d, want 404", code)
	}
	if code, _ := call(t, mux, tok, http.MethodPost, "/api/admin/scraper/smoke1/host", `{"authority":"bogus"}`); code != http.StatusBadRequest {
		t.Errorf("bad authority: %d, want 400", code)
	}
}

// TestWireUpstreamDown503 is the §16 "503 while daemon down" row: every
// proxied route answers 503 {"error": UnavailableMessage} while the stream
// is away, and the list keeps serving the mirror.
func TestWireUpstreamDown503(t *testing.T) {
	f := newWireFixture(t)
	mux, tok := newMux(t, f.adapter, f.adapter)
	f.setUp(false)

	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/admin/scraper/start", `{"name":"box1","sock":"/run/box1.sock"}`},
		{http.MethodPost, "/api/admin/scraper/smoke1/stop", ""},
		{http.MethodGet, "/api/admin/scraper/smoke1/inspect", ""},
		{http.MethodGet, "/api/admin/scraper/smoke1/diagnostics", ""},
		{http.MethodGet, "/api/admin/scraper/smoke1/host", ""},
		{http.MethodPost, "/api/admin/scraper/smoke1/host", `{"authority":"admin"}`},
	} {
		code, body := call(t, mux, tok, tc.method, tc.path, tc.body)
		if code != http.StatusServiceUnavailable || body["error"] != UnavailableMessage {
			t.Errorf("%s %s: %d %v, want 503 %q", tc.method, tc.path, code, body, UnavailableMessage)
		}
	}
	if path, _, _ := f.daemon.last(); path != "" {
		t.Errorf("daemon was called while down: %s", path)
	}
	if code, _ := call(t, mux, tok, http.MethodGet, "/api/admin/scraper", ""); code != http.StatusOK {
		t.Errorf("list while down: %d, want 200 (mirror)", code)
	}

	// Back up: the same routes work again without a rebuild.
	f.setUp(true)
	if code, _ := call(t, mux, tok, http.MethodGet, "/api/admin/scraper/smoke1/host", ""); code != http.StatusOK {
		t.Errorf("host after recovery: %d, want 200", code)
	}
}

// TestInProcessDiagnosticsUnchanged pins the pre-F6b composition for the
// in-process sources (HostControl + MapSource; no readout / health wired).
func TestInProcessDiagnosticsUnchanged(t *testing.T) {
	mux, tok := newMux(t, &inProcessScraper{FakeScraper: &pbtest.FakeScraper{}}, stubHost{})
	code, body := call(t, mux, tok, http.MethodGet, "/api/admin/scraper/box1/diagnostics", "")
	if code != http.StatusOK || body["highlighted_map"] != "B" || body["host_health"] != nil {
		t.Fatalf("diagnostics: %d %v", code, body)
	}
	if code, _ := call(t, mux, tok, http.MethodGet, "/api/admin/scraper/box1/host", ""); code != http.StatusOK {
		t.Errorf("host: %d, want 200", code)
	}
}
