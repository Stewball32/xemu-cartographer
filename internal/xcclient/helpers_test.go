package xcclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/xemu-cartographer/xc-scraper/wire"
)

// fixtureDir is the vendored copy of ../xc-scraper/wire/testdata (task sync-wire).
const fixtureDir = "../../sveltekit/src/lib/types/wire-fixtures"

const testToken = "feed-token"

// fixture returns the compact envelope bytes of one wire fixture.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fixtureDir, name+".json"))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, raw); err != nil {
		t.Fatalf("compact fixture %s: %v", name, err)
	}
	return buf.Bytes()
}

// envelopeRoom is the room the daemon frames an envelope on.
func envelopeRoom(env wire.Envelope) string {
	switch {
	case env.Type == wire.ClassHello:
		return ""
	case env.Instance == "":
		return wire.SummaryRoom
	case env.Type == wire.ClassEvents || env.Type == wire.ClassProbe:
		return wire.HostRoomPrefix + ":" + env.Instance
	default:
		return wire.HostRoomPrefix + ":" + env.Instance + ":" + env.Type
	}
}

// frameEnv frames compact envelope bytes the way hub.Frame does.
func frameEnv(t *testing.T, env []byte) []byte {
	t.Helper()
	var hdr wire.Envelope
	if err := json.Unmarshal(env, &hdr); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	data, err := json.Marshal(wire.Message{Type: wire.TypeScraper, Room: envelopeRoom(hdr), Payload: env})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// frame is frameEnv over a named fixture.
func frame(t *testing.T, name string) []byte {
	t.Helper()
	return frameEnv(t, fixture(t, name))
}

// mutated returns the fixture envelope with top-level fields overridden.
func mutated(t *testing.T, name string, set map[string]any) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(fixture(t, name), &m); err != nil {
		t.Fatal(err)
	}
	for k, v := range set {
		m[k] = v
	}
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// summaryFor builds a summary envelope listing the given instances.
func summaryFor(t *testing.T, ts string, names ...string) []byte {
	t.Helper()
	hosts := make([]wire.HostSummary, 0, len(names))
	for _, n := range names {
		hosts = append(hosts, wire.HostSummary{Instance: n, Phase: "idle"})
	}
	env := map[string]any{"v": 2, "type": wire.ClassSummary, "instance": "", "seq": 0, "tick": 0, "ts": ts,
		"data": wire.SummaryPayload{Hosts: hosts}}
	out, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	return frameEnv(t, out)
}

// logRecorder is a Config.Log that stays safe after the test ends.
type logRecorder struct {
	mu    sync.Mutex
	lines []string
}

func (l *logRecorder) Logf(format string, args ...any) {
	l.mu.Lock()
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
	l.mu.Unlock()
}

func (l *logRecorder) has(sub string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, s := range l.lines {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// eviction records one HubPort.EvictRoomPrefix call.
type eviction struct {
	Prefix string
	Status websocket.StatusCode
	Reason string
}

// fakeHub is a HubPort that records evictions and room sends.
type fakeHub struct {
	mu        sync.Mutex
	evictions []eviction
	sends     []string
}

func (h *fakeHub) SendToRoomRaw(room string, data []byte) {
	h.mu.Lock()
	h.sends = append(h.sends, room)
	h.mu.Unlock()
}

func (h *fakeHub) EvictRoomPrefix(prefix string, status websocket.StatusCode, reason string) int {
	h.mu.Lock()
	h.evictions = append(h.evictions, eviction{prefix, status, reason})
	h.mu.Unlock()
	return 1
}

func (h *fakeHub) evicted() []eviction {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]eviction(nil), h.evictions...)
}

func (h *fakeHub) evictedPrefix(prefix string) int {
	n := 0
	for _, e := range h.evicted() {
		if e.Prefix == prefix {
			n++
		}
	}
	return n
}

// fakeDaemon is an httptest server speaking the daemon's consumer side:
// /api/ws (token in ?token=, hello on accept, join/leave recorded) and
// GET /api/instances (configurable rows / status).
type fakeDaemon struct {
	t     *testing.T
	srv   *httptest.Server
	hello []byte

	mu        sync.Mutex
	conn      *websocket.Conn
	rows      []instanceRow
	status    int
	fetches   int
	rejectDia bool

	msgs     chan wire.Message
	connects chan struct{}
}

func newFakeDaemon(t *testing.T) *fakeDaemon {
	t.Helper()
	d := &fakeDaemon{
		t:        t,
		hello:    frame(t, "hello"),
		status:   http.StatusOK,
		msgs:     make(chan wire.Message, 1024),
		connects: make(chan struct{}, 16),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/ws", d.handleWS)
	mux.HandleFunc("GET /api/instances", d.handleInstances)
	d.srv = httptest.NewServer(mux)
	t.Cleanup(func() {
		d.closeConn()
		d.srv.Close()
	})
	return d
}

func (d *fakeDaemon) handleWS(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("token") != testToken {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	d.mu.Lock()
	reject := d.rejectDia
	d.mu.Unlock()
	if reject {
		http.Error(w, "down", http.StatusServiceUnavailable)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	conn.SetReadLimit(readLimit)
	d.mu.Lock()
	d.conn = conn
	hello := d.hello
	d.mu.Unlock()
	ctx := r.Context()
	if err := conn.Write(ctx, websocket.MessageText, hello); err != nil {
		return
	}
	d.connects <- struct{}{}
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			break
		}
		var m wire.Message
		if json.Unmarshal(data, &m) == nil {
			d.msgs <- m
		}
	}
	d.mu.Lock()
	if d.conn == conn {
		d.conn = nil
	}
	d.mu.Unlock()
}

func (d *fakeDaemon) handleInstances(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	d.fetches++
	rows, status := d.rows, d.status
	d.mu.Unlock()
	if got := r.Header.Get("Authorization"); got != "Bearer "+testToken {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if status != http.StatusOK {
		http.Error(w, "boom", status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rows)
}

func (d *fakeDaemon) setRows(status int, rows ...instanceRow) {
	d.mu.Lock()
	d.status, d.rows = status, rows
	d.mu.Unlock()
}

func (d *fakeDaemon) fetchCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.fetches
}

func (d *fakeDaemon) setHello(raw []byte) {
	d.mu.Lock()
	d.hello = raw
	d.mu.Unlock()
}

func (d *fakeDaemon) setReject(v bool) {
	d.mu.Lock()
	d.rejectDia = v
	d.mu.Unlock()
}

// push sends a frame to the connected client.
func (d *fakeDaemon) push(raw []byte) {
	d.t.Helper()
	d.mu.Lock()
	conn := d.conn
	d.mu.Unlock()
	if conn == nil {
		d.t.Fatal("push: no client connected")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, raw); err != nil {
		d.t.Fatalf("push: %v", err)
	}
}

// closeConn drops the current client connection server-side.
func (d *fakeDaemon) closeConn() {
	d.mu.Lock()
	conn := d.conn
	d.conn = nil
	d.mu.Unlock()
	if conn != nil {
		_ = conn.Close(websocket.StatusGoingAway, "bye")
	}
}

// waitConnect blocks until the daemon accepted a client and sent hello.
func (d *fakeDaemon) waitConnect() {
	d.t.Helper()
	select {
	case <-d.connects:
	case <-time.After(5 * time.Second):
		d.t.Fatal("client never connected")
	}
}

// drainMsgs collects control messages until want rooms of type typ were
// all seen or the timeout hits; returns every message seen.
func (d *fakeDaemon) waitRooms(typ string, want ...string) []wire.Message {
	d.t.Helper()
	pending := make(map[string]bool, len(want))
	for _, r := range want {
		pending[r] = true
	}
	var seen []wire.Message
	deadline := time.After(5 * time.Second)
	for len(pending) > 0 {
		select {
		case m := <-d.msgs:
			seen = append(seen, m)
			if m.Type == typ {
				delete(pending, m.Room)
			}
		case <-deadline:
			d.t.Fatalf("waiting for %s of %v; still pending %v; seen %v", typ, want, keys(pending), seen)
		}
	}
	return seen
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// expectNoMsg asserts no control message arrives within d.
func (d *fakeDaemon) expectNoMsg(wait time.Duration) {
	d.t.Helper()
	select {
	case m := <-d.msgs:
		d.t.Fatalf("unexpected upstream message %s %s", m.Type, m.Room)
	case <-time.After(wait):
	}
}

// newTestClient builds a client against d, tuned for fast tests, and runs
// it until the test ends.
func newTestClient(t *testing.T, d *fakeDaemon, hub HubPort, mutate func(*Client)) (*Client, *logRecorder) {
	t.Helper()
	logs := &logRecorder{}
	c, err := New(Config{URL: d.srv.URL, Token: testToken, Hub: hub, Log: logs.Logf, StaleAfter: 30 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	c.backoffMin = 20 * time.Millisecond
	c.backoffMax = 40 * time.Millisecond
	c.fetchRetry = 0
	if mutate != nil {
		mutate(c)
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

// waitFor polls cond until it holds or the timeout elapses.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}
