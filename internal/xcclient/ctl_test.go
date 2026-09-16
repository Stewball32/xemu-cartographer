package xcclient

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xemu-cartographer/xc-scraper/hostrunner"
	"github.com/xemu-cartographer/xc-scraper/runner"
)

// ctlRecorder is a daemon stand-in that records every request and answers
// from a per-route table.
type ctlRecorder struct {
	srv *httptest.Server
	mu  sync.Mutex
	got []ctlCall
	// answers by "METHOD path" → (status, body); default 200 {}
	answers map[string]ctlAnswer
	hits    atomic.Int64
	delay   time.Duration
}

type ctlCall struct {
	Method, Path, Query, Auth, ContentType string
	Body                                   []byte
}

type ctlAnswer struct {
	status int
	body   string
}

func newCtlRecorder(t *testing.T) *ctlRecorder {
	t.Helper()
	r := &ctlRecorder{answers: map[string]ctlAnswer{}}
	r.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := io.ReadAll(req.Body)
		r.hits.Add(1)
		r.mu.Lock()
		r.got = append(r.got, ctlCall{req.Method, req.URL.Path, req.URL.RawQuery,
			req.Header.Get("Authorization"), req.Header.Get("Content-Type"), body})
		ans, ok := r.answers[req.Method+" "+req.URL.Path]
		delay := r.delay
		r.mu.Unlock()
		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-req.Context().Done():
				return
			}
		}
		if !ok {
			ans = ctlAnswer{http.StatusOK, "{}"}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(ans.status)
		_, _ = io.WriteString(w, ans.body)
	}))
	t.Cleanup(r.srv.Close)
	return r
}

func (r *ctlRecorder) answer(method, path string, status int, body string) {
	r.mu.Lock()
	r.answers[method+" "+path] = ctlAnswer{status, body}
	r.mu.Unlock()
}

func (r *ctlRecorder) last(t *testing.T) ctlCall {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.got) == 0 {
		t.Fatal("no request recorded")
	}
	return r.got[len(r.got)-1]
}

func (r *ctlRecorder) ctl() *Ctl {
	return &Ctl{Base: r.srv.URL + "/", Token: "feed", ControlToken: "ctl"}
}

// TestCtlFailFastWhenDown: a Ctl bound to an idle (never connected) Client
// answers ErrUpstreamDown for every call without an HTTP round trip, and
// does so in well under a millisecond per call.
func TestCtlFailFastWhenDown(t *testing.T) {
	rec := newCtlRecorder(t)
	c, err := New(Config{URL: rec.srv.URL, Token: "feed"})
	if err != nil {
		t.Fatal(err)
	}
	ctl := NewCtl(c, "ctl")
	if ctl.Base != rec.srv.URL || ctl.Token != "feed" || ctl.ControlToken != "ctl" {
		t.Fatalf("NewCtl fields = %+v", ctl)
	}
	ctx := context.Background()
	calls := []func() error{
		func() error { _, err := ctl.Inspect(ctx, "a"); return err },
		func() error { _, err := ctl.Events(ctx, "a", 0, nil); return err },
		func() error { _, err := ctl.Probe(ctx, "a"); return err },
		func() error { _, err := ctl.Maps(ctx, "a"); return err },
		func() error { _, err := ctl.Readout(ctx, "a"); return err },
		func() error { _, err := ctl.HostHealth(ctx, "a"); return err },
		func() error { _, err := ctl.Diagnostics(ctx, "a"); return err },
		func() error { _, err := ctl.HostStatus(ctx, "a"); return err },
		func() error { _, err := ctl.Instances(ctx); return err },
		func() error { _, err := ctl.Attach(ctx, "a", "unix:/tmp/a.sock"); return err },
		func() error { return ctl.Detach(ctx, "a") },
		func() error { return ctl.Restart(ctx, "a") },
		func() error { _, err := ctl.SetAuthority(ctx, "a", "admin"); return err },
		func() error { _, err := ctl.SetSelection(ctx, "a", "m", "g"); return err },
		func() error { _, err := ctl.ClearSelection(ctx, "a"); return err },
		func() error { _, err := ctl.SetReady(ctx, "a", true); return err },
		func() error { _, err := ctl.GetConfig(ctx); return err },
		func() error { _, _, err := ctl.Do(ctx, http.MethodGet, "/api/health", nil, false); return err },
	}
	const rounds = 20
	start := time.Now()
	for i := 0; i < rounds; i++ {
		for _, fn := range calls {
			if err := fn(); !errors.Is(err, ErrUpstreamDown) {
				t.Fatalf("call %d: err = %v, want ErrUpstreamDown", i, err)
			}
		}
	}
	perCall := time.Since(start) / time.Duration(rounds*len(calls))
	if perCall >= time.Millisecond {
		t.Fatalf("fail-fast call took %v on average, want < 1ms", perCall)
	}
	if n := rec.hits.Load(); n != 0 {
		t.Fatalf("daemon saw %d request(s) while down, want 0", n)
	}
}

// TestCtlTokensAndTimeout: read calls carry the feed token, control calls
// the control token, JSON bodies set Content-Type, and the per-call
// timeout cuts a stalled daemon off.
func TestCtlTokensAndTimeout(t *testing.T) {
	rec := newCtlRecorder(t)
	ctl := rec.ctl()
	ctx := context.Background()

	rec.answer("GET", "/api/instances/a/maps", 200, `{"maps":[{"name":"Blood Gulch"}],"gametypes":[]}`)
	ml, err := ctl.Maps(ctx, "a")
	if err != nil {
		t.Fatal(err)
	}
	if len(ml.Maps) != 1 || ml.Maps[0].Name != "Blood Gulch" {
		t.Fatalf("maps = %+v", ml)
	}
	if c := rec.last(t); c.Auth != "Bearer feed" || c.ContentType != "" || c.Path != "/api/instances/a/maps" {
		t.Fatalf("read call = %+v", c)
	}

	rec.answer("PUT", "/api/ctl/instances/a/host/ready", 200, `{"instance":"a","present":true,"authority":"runner","ready":true}`)
	st, err := ctl.SetReady(ctx, "a", true)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Present || !st.Ready {
		t.Fatalf("status = %+v", st)
	}
	c := rec.last(t)
	if c.Auth != "Bearer ctl" || c.ContentType != "application/json" || string(c.Body) != `{"ready":true}` {
		t.Fatalf("control call = %+v body=%s", c, c.Body)
	}

	// Detach sends no body and no Content-Type.
	if err := ctl.Detach(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if c := rec.last(t); c.Method != "DELETE" || c.Path != "/api/ctl/instances/a" || c.ContentType != "" || len(c.Body) != 0 {
		t.Fatalf("detach call = %+v", c)
	}

	// Timeout.
	rec.mu.Lock()
	rec.delay = 300 * time.Millisecond
	rec.mu.Unlock()
	ctl.Timeout = 30 * time.Millisecond
	start := time.Now()
	_, err = ctl.Inspect(ctx, "a")
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("stalled inspect err = %v, want deadline exceeded", err)
	}
	if el := time.Since(start); el > 250*time.Millisecond {
		t.Fatalf("stalled inspect took %v, want ~30ms", el)
	}
}

// TestCtlErrorDecoding: the daemon's {"error":{"code","message"}} becomes
// *Error with the HTTP status; a bare body keeps Code "".
func TestCtlErrorDecoding(t *testing.T) {
	rec := newCtlRecorder(t)
	ctl := rec.ctl()
	ctx := context.Background()

	rec.answer("POST", "/api/ctl/attach", 409, `{"error":{"code":"already_running","message":"a is running"}}`)
	_, err := ctl.Attach(ctx, "a", "unix:/tmp/a.sock")
	var de *Error
	if !errors.As(err, &de) {
		t.Fatalf("err = %T %v, want *Error", err, err)
	}
	if de.Status != 409 || de.Code != "already_running" || de.Message != "a is running" || de.Method != "POST" {
		t.Fatalf("decoded = %+v", de)
	}
	if !errors.Is(err, &Error{Status: 409}) || !errors.Is(err, &Error{Code: "already_running"}) || errors.Is(err, &Error{Status: 404}) {
		t.Fatalf("errors.Is matching wrong for %v", err)
	}
	if c := rec.last(t); string(c.Body) != `{"name":"a","addr":"unix:/tmp/a.sock"}` {
		t.Fatalf("attach body = %s", c.Body)
	}

	rec.answer("GET", "/api/instances/b/probe", 502, `upstream broke`)
	_, err = ctl.Probe(ctx, "b")
	if !errors.As(err, &de) || de.Status != 502 || de.Code != "" || de.Message != "upstream broke" {
		t.Fatalf("bare-body err = %v", err)
	}
	if err.Error() != "xcclient: GET /api/instances/b/probe: 502 http_502: upstream broke" {
		t.Fatalf("Error() = %q", err.Error())
	}
}

// TestCtlTypedCalls: events bytes verbatim with the query encoded, attach
// 202 decoded, host selection carries exactly {map, gametype}, inspect
// decodes runner.InspectState.
func TestCtlTypedCalls(t *testing.T) {
	rec := newCtlRecorder(t)
	ctl := rec.ctl()
	ctx := context.Background()

	framed := string(frame(t, "events"))
	rec.answer("GET", "/api/instances/smoke1/events", 200, framed)
	raw, err := ctl.Events(ctx, "smoke1", 42, []string{"death", "medal"})
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != framed {
		t.Fatalf("events bytes changed:\n got %s\nwant %s", raw, framed)
	}
	if c := rec.last(t); c.Query != "since_tick=42&types=death%2Cmedal" {
		t.Fatalf("events query = %q", c.Query)
	}
	if _, err := ctl.Events(ctx, "smoke1", 0, nil); err != nil {
		t.Fatal(err)
	}
	if c := rec.last(t); c.Query != "" {
		t.Fatalf("zero-arg events query = %q, want empty", c.Query)
	}

	rec.answer("POST", "/api/ctl/attach", 202, `{"name":"smoke1","addr":"unix:/tmp/s.sock","phase":"attaching"}`)
	resp, err := ctl.Attach(ctx, "smoke1", "unix:/tmp/s.sock")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Name != "smoke1" || resp.Phase != "attaching" {
		t.Fatalf("attach resp = %+v", resp)
	}

	rec.answer("PUT", "/api/ctl/instances/smoke1/host/selection", 200, `{"instance":"smoke1","present":true,"authority":"runner","selected":true}`)
	if _, err := ctl.SetSelection(ctx, "smoke1", "Blood Gulch", "Slayer"); err != nil {
		t.Fatal(err)
	}
	var sel map[string]any
	if err := json.Unmarshal(rec.last(t).Body, &sel); err != nil {
		t.Fatal(err)
	}
	if len(sel) != 2 || sel["map"] != "Blood Gulch" || sel["gametype"] != "Slayer" {
		t.Fatalf("selection body = %v, want exactly {map, gametype}", sel)
	}

	rec.answer("POST", "/api/ctl/instances/smoke1/host", 200, `{"instance":"smoke1","present":true,"authority":"admin"}`)
	st, err := ctl.SetAuthority(ctx, "smoke1", hostrunner.AuthAdmin.String())
	if err != nil || st.Authority != "admin" {
		t.Fatalf("SetAuthority = %+v, %v", st, err)
	}
	if c := rec.last(t); string(c.Body) != `{"authority":"admin"}` {
		t.Fatalf("authority body = %s", c.Body)
	}

	want := runner.InspectState{Info: runner.Info{Name: "smoke1", Sock: "/tmp/s.sock"}, Running: true, Phase: "live"}
	body, _ := json.Marshal(want)
	rec.answer("GET", "/api/instances/smoke1/inspect", 200, string(body))
	got, err := ctl.Inspect(ctx, "smoke1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "smoke1" || got.Sock != "/tmp/s.sock" || !got.Running || got.Phase != "live" {
		t.Fatalf("inspect = %+v", got)
	}

	rec.answer("GET", "/api/instances/smoke1/diagnostics", 200, `{"host_runner":{"instance":"smoke1","present":true,"tick":7},"present":true,"readout":null,"health":null,"health_age_ms":0,"maps":{"maps":[],"gametypes":[]}}`)
	d, err := ctl.Diagnostics(ctx, "smoke1")
	if err != nil || d.HostRunner.Tick != 7 || !d.Present {
		t.Fatalf("diagnostics = %+v, %v", d, err)
	}
}
