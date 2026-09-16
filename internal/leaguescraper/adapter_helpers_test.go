package leaguescraper

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/xcclient"
	"github.com/xemu-cartographer/xc-scraper/wire"
)

// wireFixtureDir is the vendored copy of ../xc-scraper/wire/testdata.
const wireFixtureDir = "../../sveltekit/src/lib/types/wire-fixtures"

// wireFixture returns the compact envelope bytes of one wire fixture.
func wireFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(wireFixtureDir, name+".json"))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		t.Fatalf("compact fixture %s: %v", name, err)
	}
	return buf.Bytes()
}

// framedFixture frames a fixture envelope the way the daemon hub does and
// returns the bytes with the decoded envelope.
func framedFixture(t *testing.T, name string) ([]byte, wire.Envelope) {
	t.Helper()
	return frameEnvelope(t, wireFixture(t, name))
}

func frameEnvelope(t *testing.T, envBytes []byte) ([]byte, wire.Envelope) {
	t.Helper()
	var env wire.Envelope
	if err := json.Unmarshal(envBytes, &env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	room := ""
	switch {
	case env.Type == wire.ClassHello:
	case env.Instance == "":
		room = wire.SummaryRoom
	case env.Type == wire.ClassEvents || env.Type == wire.ClassProbe:
		room = wire.HostRoomPrefix + ":" + env.Instance
	default:
		room = wire.HostRoomPrefix + ":" + env.Instance + ":" + env.Type
	}
	raw, err := json.Marshal(wire.Message{Type: wire.TypeScraper, Room: room, Payload: envBytes})
	if err != nil {
		t.Fatal(err)
	}
	return raw, env
}

// storeFixture feeds one state-class fixture into the adapter's mirror.
func storeFixture(t *testing.T, a *Adapter, name string) {
	t.Helper()
	raw, env := framedFixture(t, name)
	a.client.Mirror().Store(env.Instance, env.Type, raw, &env)
}

// daemonStub records requests and answers from a per-route table (default
// 200 {}).
type daemonStub struct {
	srv     *httptest.Server
	mu      sync.Mutex
	calls   []stubCall
	answers map[string]stubAnswer
	hits    atomic.Int64
	delay   time.Duration
}

type stubCall struct {
	Method, Path, Query, Auth string
	Body                      []byte
}

type stubAnswer struct {
	status int
	body   string
}

func newDaemonStub(t *testing.T) *daemonStub {
	t.Helper()
	d := &daemonStub{answers: map[string]stubAnswer{}}
	d.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		d.hits.Add(1)
		d.mu.Lock()
		d.calls = append(d.calls, stubCall{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization"), body})
		ans, ok := d.answers[r.Method+" "+r.URL.Path]
		delay := d.delay
		d.mu.Unlock()
		if delay > 0 {
			time.Sleep(delay)
		}
		if !ok {
			ans = stubAnswer{http.StatusOK, "{}"}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(ans.status)
		_, _ = io.WriteString(w, ans.body)
	}))
	t.Cleanup(d.srv.Close)
	return d
}

func (d *daemonStub) answer(method, path string, status int, body string) {
	d.mu.Lock()
	d.answers[method+" "+path] = stubAnswer{status, body}
	d.mu.Unlock()
}

func (d *daemonStub) last(t *testing.T) stubCall {
	t.Helper()
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.calls) == 0 {
		t.Fatal("daemon stub saw no request")
	}
	return d.calls[len(d.calls)-1]
}

// newTestAdapter builds an Adapter over an idle client whose Ctl talks to
// the stub without the connected gate (connected nil = always up).
func newTestAdapter(t *testing.T, d *daemonStub, connected func() bool) *Adapter {
	t.Helper()
	c, err := xcclient.New(xcclient.Config{URL: d.srv.URL, Token: "feed", Log: t.Logf})
	if err != nil {
		t.Fatal(err)
	}
	ctl := &xcclient.Ctl{Base: d.srv.URL, Token: "feed", ControlToken: "ctl", Connected: connected}
	a := NewAdapter(c, ctl)
	a.logf = t.Logf
	return a
}
