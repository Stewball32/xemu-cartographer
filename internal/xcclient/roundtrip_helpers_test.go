package xcclient

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/xemu-cartographer/xc-scraper/daemon"
	"github.com/xemu-cartographer/xc-scraper/hub"
	"github.com/xemu-cartographer/xc-scraper/runner"
	"github.com/xemu-cartographer/xc-scraper/wire"
)

// fixtureReplayer is a hub.Replayer + hub.HelloSource over the vendored wire
// fixtures: hello lists names, join replay serves the compact fixture
// envelope of each (instance, class) in frames, host:all/summary replay
// serves the summary fixture when present.
type fixtureReplayer struct {
	mu        sync.Mutex
	startedAt time.Time
	names     []string
	frames    map[string]map[string][]byte // instance → class → envelope
	summ      []byte
}

func newFixtureReplayer(t *testing.T, names ...string) *fixtureReplayer {
	t.Helper()
	return &fixtureReplayer{
		startedAt: time.Date(2026, 9, 1, 11, 0, 0, 0, time.UTC),
		names:     names,
		frames:    make(map[string]map[string][]byte),
	}
}

// serve registers the fixture envelope for the instance it names.
func (r *fixtureReplayer) serve(t *testing.T, name string) []byte {
	t.Helper()
	env := fixture(t, name)
	var hdr wire.Envelope
	if err := json.Unmarshal(env, &hdr); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if hdr.Type == wire.ClassSummary {
		r.summ = env
		return env
	}
	if r.frames[hdr.Instance] == nil {
		r.frames[hdr.Instance] = make(map[string][]byte)
	}
	r.frames[hdr.Instance][hdr.Type] = env
	return env
}

func (r *fixtureReplayer) HelloPayloadFiltered(keep func([]string) []string) wire.HelloPayload {
	r.mu.Lock()
	names := append([]string(nil), r.names...)
	startedAt := r.startedAt
	r.mu.Unlock()
	if keep != nil {
		names = keep(names)
	}
	hp := wire.HelloPayload{ProtocolVersion: wire.ProtocolVersion, ServerTime: time.Now().UTC(), Classes: wire.AllClasses()}
	hp.Instances = make([]wire.HelloInstance, 0, len(names))
	for _, n := range names {
		hp.Instances = append(hp.Instances, wire.HelloInstance{Name: n, StartedAt: startedAt})
	}
	return hp
}

// restart moves every instance's started_at (a daemon-side instance restart
// as GET /api/instances and the next hello would report it).
func (r *fixtureReplayer) restart(at time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.startedAt = at
}

func (r *fixtureReplayer) JoinReplayForInstance(name string) []runner.Reply {
	var out []runner.Reply
	for _, class := range wire.StateClasses() {
		out = append(out, r.JoinReplayForInstanceClass(name, class)...)
	}
	return out
}

func (r *fixtureReplayer) JoinReplayForInstanceClass(name, class string) []runner.Reply {
	r.mu.Lock()
	defer r.mu.Unlock()
	env := r.frames[name][class]
	if env == nil {
		return nil
	}
	return []runner.Reply{{Instance: name, Class: class, Envelope: env}}
}

func (r *fixtureReplayer) JoinReplayForHostAll() []runner.Reply {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.summ == nil {
		return nil
	}
	return []runner.Reply{{Class: wire.ClassSummary, Envelope: r.summ}}
}

func (r *fixtureReplayer) EventsReply(string, uint32, []string) (runner.Reply, bool) {
	return runner.Reply{}, false
}

func (r *fixtureReplayer) ProbeReply(string) (runner.Reply, bool) { return runner.Reply{}, false }

// instancesJSON is the GET /api/instances body the client's placeholder
// fetch reads: one row per hello instance.
func (r *fixtureReplayer) instancesJSON() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	rows := make([]instanceRow, 0, len(r.names))
	for _, n := range r.names {
		rows = append(rows, instanceRow{Name: n, StartedAt: r.startedAt})
	}
	out, _ := json.Marshal(rows)
	return out
}

// upstream is an xc/hub served on 127.0.0.1:0 the way the daemon mounts it:
// /api/ws plus GET /api/instances.
type upstream struct {
	hub *hub.Hub
	rp  *fixtureReplayer
	srv *http.Server
	url string
}

func newUpstream(t *testing.T, rp *fixtureReplayer) *upstream {
	t.Helper()
	return newUpstreamOn(t, rp, "127.0.0.1:0", testToken)
}

// newUpstreamOn is newUpstream on a fixed listen address with its own feed
// token, so a test can restart "the same daemon" (reconnect) or swap the
// token it checks.
func newUpstreamOn(t *testing.T, rp *fixtureReplayer, addr, token string) *upstream {
	t.Helper()
	h := hub.New(hub.Config{Token: token, AllowedOrigins: []string{"127.0.0.1"}}, rp, hub.HelloHook(rp))
	mux := http.NewServeMux()
	mux.Handle("/api/ws", h.Handler())
	mux.HandleFunc("GET /api/instances", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, `{"error":{"code":"unauthorized"}}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(rp.instancesJSON())
	})
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	u := &upstream{hub: h, rp: rp, srv: &http.Server{Handler: mux}, url: "http://" + ln.Addr().String()}
	go h.Run()
	go func() { _ = u.srv.Serve(ln) }()
	t.Cleanup(u.stop)
	return u
}

// stop tears the upstream down (idempotent) so the client sees a disconnect.
func (u *upstream) stop() {
	u.hub.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = u.srv.Shutdown(ctx)
	_ = u.srv.Close()
}

// bootDaemon boots a real xc/daemon (AllowEmpty, temp state dirs) on
// 127.0.0.1:0 and returns its base URL plus a stop func.
func bootDaemon(t *testing.T) (base string, s *daemon.Server, stop func()) {
	t.Helper()
	cfg := daemon.Config{
		Listen:     "127.0.0.1:0",
		Token:      testToken,
		AllowEmpty: true,
		StateDir:   t.TempDir(),
		OffsetsDir: t.TempDir(),
		Version:    "roundtrip-test",
	}
	s, err := daemon.New(cfg)
	if err != nil {
		t.Fatalf("daemon.New: %v", err)
	}
	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = s.Serve(ctx, ln)
	}()
	var once sync.Once
	stop = func() {
		once.Do(func() {
			cancel()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Error("daemon did not stop")
			}
		})
	}
	t.Cleanup(stop)
	return "http://" + ln.Addr().String(), s, stop
}

// send is one HubPort.SendToRoomRaw call.
type send struct {
	Room string
	Data []byte
}

// sinkHub is a HubPort recording sends (room + bytes) and evictions.
type sinkHub struct {
	mu        sync.Mutex
	sends     []send
	evictions []eviction
}

func (h *sinkHub) SendToRoomRaw(room string, data []byte) {
	h.mu.Lock()
	h.sends = append(h.sends, send{Room: room, Data: append([]byte(nil), data...)})
	h.mu.Unlock()
}

func (h *sinkHub) EvictRoomPrefix(prefix string, status websocket.StatusCode, reason string) int {
	h.mu.Lock()
	h.evictions = append(h.evictions, eviction{prefix, status, reason})
	h.mu.Unlock()
	return 1
}

// sent returns every send on room, in order.
func (h *sinkHub) sent(room string) [][]byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out [][]byte
	for _, s := range h.sends {
		if s.Room == room {
			out = append(out, s.Data)
		}
	}
	return out
}

func (h *sinkHub) rooms() map[string]int {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make(map[string]int)
	for _, s := range h.sends {
		out[s.Room]++
	}
	return out
}

func (h *sinkHub) evicted() []eviction {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]eviction(nil), h.evictions...)
}

// evictedPrefix counts evictions on prefix.
func (h *sinkHub) evictedPrefix(prefix string) int {
	n := 0
	for _, e := range h.evicted() {
		if e.Prefix == prefix {
			n++
		}
	}
	return n
}

// bootClient runs a client against base with fast tunables and returns it
// with its log recorder; Run stops on test cleanup.
func bootClient(t *testing.T, base string, mutate func(*Config), tune func(*Client)) (*Client, *logRecorder) {
	t.Helper()
	logs := &logRecorder{}
	cfg := Config{URL: base, Token: testToken, Log: logs.Logf, StaleAfter: 30 * time.Second}
	if mutate != nil {
		mutate(&cfg)
	}
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	c.backoffMin = 20 * time.Millisecond
	c.backoffMax = 40 * time.Millisecond
	c.fetchRetry = 0
	if tune != nil {
		tune(c)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = c.Run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("Run did not stop")
		}
	})
	return c, logs
}
