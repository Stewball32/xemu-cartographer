package scraper

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/authz/pb/pbtest"
	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/leaguescraper"
	"github.com/Stewball32/xemu-cartographer/internal/xcclient"
	"github.com/xemu-cartographer/xc-scraper/hosthealth"
	"github.com/xemu-cartographer/xc-scraper/hostrunner"
	"github.com/xemu-cartographer/xc-scraper/runner"
)

const (
	testFeedToken    = "feed-secret"
	testControlToken = "ctl-secret"
)

// fakeDaemon is a stub xc-scraper HTTP surface: the read + control routes the
// F6b proxies touch, answering the documented bodies / error codes for the
// fixed instance "smoke1" (attached) and rejecting the rest. It records the
// last control body so the tests can assert what travelled.
type fakeDaemon struct {
	srv *httptest.Server

	mu       sync.Mutex
	lastPath string
	lastBody map[string]any
	lastAuth string
}

func newFakeDaemon(t *testing.T) *fakeDaemon {
	t.Helper()
	d := &fakeDaemon{}
	mux := http.NewServeMux()
	writeErr := func(w http.ResponseWriter, status int, code, msg string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": msg}})
	}
	writeJSON := func(w http.ResponseWriter, status int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
	record := func(r *http.Request) map[string]any {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		d.mu.Lock()
		d.lastPath, d.lastBody, d.lastAuth = r.URL.Path, body, r.Header.Get("Authorization")
		d.mu.Unlock()
		return body
	}
	ctl := func(fn http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer "+testControlToken {
				writeErr(w, http.StatusUnauthorized, "unauthorized", "bad control token")
				return
			}
			fn(w, r)
		}
	}
	attached := func(w http.ResponseWriter, r *http.Request) (string, bool) {
		name := r.PathValue("name")
		if name != "smoke1" {
			writeErr(w, http.StatusNotFound, "not_found", "no runner attached for "+name)
			return "", false
		}
		return name, true
	}
	status := func(name string) hostrunner.Status {
		return hostrunner.Status{Instance: name, Present: true, Authority: "runner", SelectedMap: "Blood Gulch"}
	}

	mux.HandleFunc("GET /api/instances/{name}/inspect", func(w http.ResponseWriter, r *http.Request) {
		if name, ok := attached(w, r); ok {
			writeJSON(w, http.StatusOK, runner.InspectState{Info: runner.Info{Name: name}, Phase: "ready"})
		}
	})
	mux.HandleFunc("GET /api/instances/{name}/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		name, ok := attached(w, r)
		if !ok {
			return
		}
		hh := hosthealth.Health{}
		writeJSON(w, http.StatusOK, map[string]any{
			"host_runner": hostrunner.Diagnostics{
				Instance: name, Present: true, Authority: "runner",
				MapCursor:      hostrunner.CursorView{Index: 1, Count: 3, Valid: true},
				GametypeCursor: hostrunner.CursorView{Index: 0, Count: 2, Valid: true},
			},
			"present":       true,
			"readout":       nil,
			"health":        hh,
			"health_age_ms": 250,
			"maps": runner.MapList{Available: true,
				Maps:      []runner.MapOption{{Name: "Battle Creek"}, {Name: "Blood Gulch", Steps: 1}, {Name: "Chill Out", Steps: 2}},
				Gametypes: []runner.MapOption{{Name: "Slayer"}, {Name: "CTF", Steps: 1}}},
		})
	})
	mux.HandleFunc("GET /api/instances/{name}/host", func(w http.ResponseWriter, r *http.Request) {
		if name, ok := attached(w, r); ok {
			writeJSON(w, http.StatusOK, status(name))
		}
	})
	mux.HandleFunc("POST /api/ctl/attach", ctl(func(w http.ResponseWriter, r *http.Request) {
		body := record(r)
		name, _ := body["name"].(string)
		switch name {
		case "dup":
			writeErr(w, http.StatusConflict, "already_running", "dup is already running")
		case "bad name":
			writeErr(w, http.StatusBadRequest, "invalid_name", "invalid instance name")
		default:
			writeJSON(w, http.StatusAccepted, map[string]any{"name": name, "addr": body["addr"], "phase": "attaching"})
		}
	}))
	mux.HandleFunc("DELETE /api/ctl/instances/{name}", ctl(func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if name, ok := attached(w, r); ok {
			writeJSON(w, http.StatusOK, map[string]any{"name": name, "detached": true})
		}
	}))
	mux.HandleFunc("POST /api/ctl/instances/{name}/host", ctl(func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if name, ok := attached(w, r); ok {
			writeJSON(w, http.StatusOK, status(name))
		}
	}))
	mux.HandleFunc("PUT /api/ctl/instances/{name}/host/selection", ctl(func(w http.ResponseWriter, r *http.Request) {
		body := record(r)
		name, ok := attached(w, r)
		if !ok {
			return
		}
		switch body["map"] {
		case "Nowhere":
			writeErr(w, http.StatusConflict, "not_in_carousel", "map not available on this instance: Nowhere")
		case "Unread":
			writeErr(w, http.StatusNotFound, "no_maps", "carousel not enumerated yet for "+name)
		default:
			st := status(name)
			st.SelectedMap, _ = body["map"].(string)
			st.SelectedGametype, _ = body["gametype"].(string)
			writeJSON(w, http.StatusOK, st)
		}
	}))
	mux.HandleFunc("POST /api/ctl/xemu/{tool}", ctl(func(w http.ResponseWriter, r *http.Request) {
		body := record(r)
		addr, _ := body["addr"].(string)
		if !strings.HasSuffix(addr, "/smoke1.sock") {
			writeErr(w, http.StatusNotFound, "not_attached", addr+" is not an attached instance")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"addr": addr, "sock": strings.TrimPrefix(addr, "unix:"), "tool": r.PathValue("tool"), "echo": body})
	}))
	d.srv = httptest.NewServer(mux)
	t.Cleanup(d.srv.Close)
	return d
}

// last returns the most recent control call (path, decoded body, auth header).
func (d *fakeDaemon) last() (string, map[string]any, string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.lastPath, d.lastBody, d.lastAuth
}

// wireFixture is a real leaguescraper.Adapter bound to the fake daemon. The
// stream client is constructed but never run, so the Ctl's fail-fast gate is
// overridden with `up` — flip it to simulate the daemon going away.
type wireFixture struct {
	daemon  *fakeDaemon
	adapter *leaguescraper.Adapter
	ctl     *xcclient.Ctl
	mu      sync.Mutex
	up      bool
}

func newWireFixture(t *testing.T) *wireFixture {
	t.Helper()
	d := newFakeDaemon(t)
	c, err := xcclient.New(xcclient.Config{URL: d.srv.URL, Token: testFeedToken})
	if err != nil {
		t.Fatalf("xcclient.New: %v", err)
	}
	f := &wireFixture{daemon: d, up: true}
	f.ctl = xcclient.NewCtl(c, testControlToken)
	f.ctl.Connected = func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.up
	}
	f.adapter = leaguescraper.NewAdapter(c, f.ctl)
	return f
}

func (f *wireFixture) setUp(up bool) {
	f.mu.Lock()
	f.up = up
	f.mu.Unlock()
}

// inProcessScraper is the in-process fixture: pbtest.FakeScraper whose Start
// makes the instance visible in List (like runner.Manager) so POST /start
// answers 201 with the Info.
type inProcessScraper struct {
	*pbtest.FakeScraper
	startErr error
	stopErr  error
}

func (s *inProcessScraper) Start(name, sock string) error {
	if s.startErr != nil {
		return s.startErr
	}
	s.AddInstance(name, "xbox-"+name)
	return nil
}

func (s *inProcessScraper) Stop(name string) error { return s.stopErr }

// stubHost is the in-process HostControl / sources stand-in.
type stubHost struct{}

func (stubHost) Status(instance string) hostrunner.Status {
	return hostrunner.Status{Instance: instance, Present: true, Authority: "runner"}
}
func (stubHost) SetAuthority(string, hostrunner.Authority) bool { return true }
func (stubHost) Diagnostics(instance string) hostrunner.Diagnostics {
	return hostrunner.Diagnostics{Instance: instance, MapCursor: hostrunner.CursorView{Index: 1, Count: 2}}
}
func (stubHost) AvailableMaps(string) scraperiface.MapList {
	return scraperiface.MapList{Available: true, Maps: []runner.MapOption{{Name: "A"}, {Name: "B", Steps: 1}}}
}

// newMux mounts the scraper group on the real PocketBase router with src as
// every injected source (Manager + HostRunners + Maps; Readouts / Health only
// when src implements them) and returns the mux plus a superuser bearer.
func newMux(t *testing.T, src scraperiface.Service, host HostControl) (http.Handler, string) {
	t.Helper()
	app, _ := pbtest.NewApp(t)
	su := pbtest.NewSuperuser(t, app, "admin@example.com")
	tok, err := su.NewAuthToken()
	if err != nil {
		t.Fatalf("NewAuthToken: %v", err)
	}
	SetManager(src)
	SetHostControl(host)
	Maps, Readouts, Health = nil, nil, nil
	if m, ok := host.(MapSource); ok {
		SetMapSource(m)
	}
	if r, ok := host.(ReadoutSource); ok {
		SetReadoutSource(r)
	}
	if h, ok := host.(HealthSource); ok {
		SetHealthSource(h)
	}
	t.Cleanup(func() {
		Manager, HostRunners, Maps, Readouts, Health = nil, nil, nil, nil, nil
	})
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

// call performs one admin request and returns the status + decoded body.
func call(t *testing.T, mux http.Handler, tok, method, path, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", tok)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	var out map[string]any
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec.Code, out
}
