package xcclient

import (
	"encoding/json"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/xemu-cartographer/xc-scraper/wire"
)

func TestEndpoints(t *testing.T) {
	cases := []struct {
		in, token, base, ws string
		wantErr             bool
	}{
		{in: "http://127.0.0.1:8990", token: "t", base: "http://127.0.0.1:8990", ws: "ws://127.0.0.1:8990/api/ws?token=t"},
		{in: "https://xc.example.com/", base: "https://xc.example.com", ws: "wss://xc.example.com/api/ws"},
		{in: "http://host:1/prefix/", token: "a b", base: "http://host:1/prefix", ws: "ws://host:1/prefix/api/ws?token=a+b"},
		{in: "ws://host:2", base: "http://host:2", ws: "ws://host:2/api/ws"},
		{in: "", wantErr: true},
		{in: "ftp://host", wantErr: true},
		{in: "http://", wantErr: true},
		{in: "127.0.0.1:8990", wantErr: true},
	}
	for _, tc := range cases {
		base, ws, err := Endpoints(tc.in, tc.token)
		if tc.wantErr {
			if err == nil {
				t.Errorf("Endpoints(%q): want error, got %s %s", tc.in, base, ws)
			}
			continue
		}
		if err != nil {
			t.Errorf("Endpoints(%q): %v", tc.in, err)
			continue
		}
		if base.String() != tc.base || ws != tc.ws {
			t.Errorf("Endpoints(%q) = %s, %s; want %s, %s", tc.in, base, ws, tc.base, tc.ws)
		}
	}
	if _, err := New(Config{URL: "nope"}); err == nil {
		t.Fatal("New accepted a bad URL")
	}
}

// helloRooms is the join set the hello fixture (smoke1, smoke2) implies.
func helloRooms(extra ...string) []string {
	rooms := []string{wire.SummaryRoom}
	rooms = append(rooms, alwaysRooms("smoke1")...)
	rooms = append(rooms, alwaysRooms("smoke2")...)
	return append(rooms, extra...)
}

func TestStreamHelloJoinsAndEvicts(t *testing.T) {
	d := newFakeDaemon(t)
	hub := &fakeHub{}
	c, logs := newTestClient(t, d, hub, func(c *Client) { c.cfg.Hostrunner = true })
	d.waitConnect()
	seen := d.waitRooms(wire.TypeJoinRoom, helloRooms(HostrunnerRoom)...)
	for _, m := range seen {
		if m.Type != wire.TypeJoinRoom {
			t.Fatalf("unexpected %s %s during connect", m.Type, m.Room)
		}
	}
	if got := c.Mirror().Instances(); !reflect.DeepEqual(got, []string{"smoke1", "smoke2"}) {
		t.Fatalf("Instances = %v", got)
	}
	want := time.Date(2026, 9, 1, 11, 42, 10, 0, time.UTC)
	if at, ok := c.Mirror().StartedAt("smoke1"); !ok || !at.Equal(want) {
		t.Fatalf("StartedAt(smoke1) = %v ok=%v", at, ok)
	}
	waitFor(t, "connect eviction", func() bool { return hub.evictedPrefix("host:") == 1 })
	if ev := hub.evicted()[0]; ev.Status != websocket.StatusServiceRestart || ev.Reason != EvictReason {
		t.Fatalf("eviction = %+v", ev)
	}
	st := c.Status()
	if !st.Connected || st.Reconnects != 0 || st.Stale || st.LastFrameAt.IsZero() || st.Since.IsZero() {
		t.Fatalf("Status = %+v", st)
	}
	if !logs.has("hello: 2 instance(s)") {
		t.Fatal("hello not logged")
	}
	if d.fetchCount() != 0 {
		t.Fatal("hello instances must not trigger GET /api/instances")
	}
}

func TestStreamSummaryAddFetchesStartedAt(t *testing.T) {
	d := newFakeDaemon(t)
	hub := &fakeHub{}
	c, _ := newTestClient(t, d, hub, nil)
	d.waitConnect()
	d.waitRooms(wire.TypeJoinRoom, helloRooms()...)

	real := time.Date(2026, 9, 1, 11, 50, 0, 0, time.UTC)
	d.setRows(http.StatusOK, instanceRow{Name: "smoke3", StartedAt: real})
	d.push(summaryFor(t, "2026-09-01T12:00:00Z", "smoke1", "smoke2", "smoke3"))
	d.waitRooms(wire.TypeJoinRoom, alwaysRooms("smoke3")...)
	if at, ok := c.Mirror().StartedAt("smoke3"); !ok || !at.Equal(real) {
		t.Fatalf("StartedAt(smoke3) = %v ok=%v", at, ok)
	}
	if got := c.Mirror().Placeholders(); len(got) != 0 {
		t.Fatalf("Placeholders = %v", got)
	}
	waitFor(t, "instance eviction", func() bool { return hub.evictedPrefix("host:smoke3") == 1 })
	if sp, ok := c.Mirror().Summary(); !ok || len(sp.Hosts) != 3 {
		t.Fatalf("Summary = %+v ok=%v", sp, ok)
	}
	if got := c.Mirror().JoinReplayForHostAll(); len(got) != 1 {
		t.Fatal("summary frame not cached")
	}

	d.push(summaryFor(t, "2026-09-01T12:00:01Z", "smoke1", "smoke2"))
	d.waitRooms(wire.TypeLeaveRoom, alwaysRooms("smoke3")...)
	if got := c.Mirror().Instances(); !reflect.DeepEqual(got, []string{"smoke1", "smoke2"}) {
		t.Fatalf("Instances after removal = %v", got)
	}
}

func TestStreamSummaryAddTsFallback(t *testing.T) {
	d := newFakeDaemon(t)
	hub := &fakeHub{}
	c, logs := newTestClient(t, d, hub, nil)
	d.waitConnect()
	d.waitRooms(wire.TypeJoinRoom, helloRooms()...)

	d.setRows(http.StatusInternalServerError)
	d.push(summaryFor(t, "2026-09-01T12:00:00Z", "smoke1", "smoke2", "smoke3"))
	d.waitRooms(wire.TypeJoinRoom, alwaysRooms("smoke3")...)
	ts := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	if at, ok := c.Mirror().StartedAt("smoke3"); !ok || !at.Equal(ts) {
		t.Fatalf("placeholder StartedAt(smoke3) = %v ok=%v", at, ok)
	}
	if got := c.Mirror().Placeholders(); !reflect.DeepEqual(got, []string{"smoke3"}) {
		t.Fatalf("Placeholders = %v", got)
	}
	if !logs.has("started_at for [smoke3] unavailable") {
		t.Fatal("fallback not logged")
	}

	real := ts.Add(-10 * time.Minute)
	d.setRows(http.StatusOK, instanceRow{Name: "smoke3", StartedAt: real})
	d.push(summaryFor(t, "2026-09-01T12:00:05Z", "smoke1", "smoke2", "smoke3"))
	waitFor(t, "placeholder replaced", func() bool {
		at, _ := c.Mirror().StartedAt("smoke3")
		return at.Equal(real)
	})
	if got := c.Mirror().Placeholders(); len(got) != 0 {
		t.Fatalf("Placeholders = %v", got)
	}
	if n := hub.evictedPrefix("host:smoke3"); n != 1 {
		t.Fatalf("host:smoke3 evictions = %d; replacing a placeholder must not evict", n)
	}
	d.expectNoMsg(50 * time.Millisecond)
}

// frameRecorder is an OnFrame that records what it saw.
type frameRecorder struct {
	mu     sync.Mutex
	frames []Frame
}

func (r *frameRecorder) OnFrame(f Frame) {
	r.mu.Lock()
	r.frames = append(r.frames, f)
	r.mu.Unlock()
}

func (r *frameRecorder) find(msgType, class string) (Frame, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, f := range r.frames {
		if f.Msg.Type != msgType {
			continue
		}
		if class == "" || (f.Env != nil && f.Env.Type == class) {
			return f, true
		}
	}
	return Frame{}, false
}

func TestStreamOnFrameErrorsAndHostRunner(t *testing.T) {
	d := newFakeDaemon(t)
	rec := &frameRecorder{}
	_, logs := newTestClient(t, d, nil, func(c *Client) { c.cfg.OnFrame = rec.OnFrame })
	d.waitConnect()
	d.waitRooms(wire.TypeJoinRoom, helloRooms()...)

	// not_found is the generic path; forbidden is the feed-token latch
	// (TestStreamForbiddenLatchesAuthRejected).
	errFrame, _ := json.Marshal(wire.Message{Type: wire.TypeError, Room: "nope:x", Payload: json.RawMessage(`{"code":"not_found","message":"unknown room type"}`)})
	hr, _ := json.Marshal(wire.Message{Type: "host_runner", Payload: json.RawMessage(`{"instance":"smoke1"}`)})
	d.push(errFrame)
	d.push(hr)
	d.push(frame(t, "game"))

	waitFor(t, "game frame", func() bool { _, ok := rec.find(wire.TypeScraper, wire.ClassGame); return ok })
	if f, ok := rec.find(wire.TypeScraper, wire.ClassHello); !ok || f.Env.Instance != "" {
		t.Fatal("hello not handed to OnFrame")
	}
	if f, ok := rec.find("host_runner", ""); !ok || f.Env != nil || string(f.Raw) != string(hr) {
		t.Fatalf("host_runner frame = %+v ok=%v", f, ok)
	}
	if _, ok := rec.find(wire.TypeError, ""); ok {
		t.Fatal("error frame handed to OnFrame")
	}
	if !logs.has("upstream error frame") {
		t.Fatal("error frame not logged")
	}
	if f, _ := rec.find(wire.TypeScraper, wire.ClassGame); f.Msg.Room != "host:smoke1:game" || string(f.Raw) != string(frame(t, "game")) {
		t.Fatalf("game frame = %s", f.Raw)
	}
}

func gameSeq(t *testing.T, seq uint64) []byte {
	t.Helper()
	return frameEnv(t, mutated(t, "game", map[string]any{"seq": seq}))
}

func TestStreamSeqRegressionIsEpoch(t *testing.T) {
	d := newFakeDaemon(t)
	hub := &fakeHub{}
	c, logs := newTestClient(t, d, hub, nil)
	d.waitConnect()
	d.waitRooms(wire.TypeJoinRoom, helloRooms()...)
	m := c.Mirror()

	d.push(frame(t, "scenario"))
	d.push(gameSeq(t, 41))
	waitFor(t, "game cached", func() bool { return m.Frame("smoke1", wire.ClassGame) != nil })
	if m.Frame("smoke1", wire.ClassScenario) == nil {
		t.Fatal("scenario not cached")
	}
	d.push(gameSeq(t, 45))
	waitFor(t, "seq gap counted", func() bool { return c.Status().SeqGaps == 3 })
	if !logs.has("seq gap of 3") {
		t.Fatal("gap not logged")
	}
	if hub.evictedPrefix("host:smoke1") != 0 || d.fetchCount() != 0 {
		t.Fatal("a gap must not be an epoch")
	}

	restarted := time.Date(2026, 9, 1, 12, 30, 0, 0, time.UTC)
	d.setRows(http.StatusOK, instanceRow{Name: "smoke1", StartedAt: restarted})
	d.push(gameSeq(t, 2))
	waitFor(t, "epoch eviction", func() bool { return hub.evictedPrefix("host:smoke1") == 1 })
	if m.Frame("smoke1", wire.ClassScenario) != nil {
		t.Fatal("epoch kept the scenario frame")
	}
	if m.Frame("smoke1", wire.ClassGame) == nil {
		t.Fatal("epoch dropped the regressing frame itself")
	}
	waitFor(t, "started_at re-read", func() bool {
		at, _ := m.StartedAt("smoke1")
		return at.Equal(restarted)
	})
	if !logs.has("seq regression on game; epoch change") {
		t.Fatal("epoch not logged")
	}
	if got := c.Status(); got.SeqGaps != 3 || got.Stale {
		t.Fatalf("Status = %+v", got)
	}
	// smoke2 is untouched by smoke1's epoch.
	if got := m.Instances(); !reflect.DeepEqual(got, []string{"smoke1", "smoke2"}) {
		t.Fatalf("Instances = %v", got)
	}
}

func TestStreamStaleClearAndReconnect(t *testing.T) {
	d := newFakeDaemon(t)
	hub := &fakeHub{}
	c, logs := newTestClient(t, d, hub, func(c *Client) { c.staleAfter = 50 * time.Millisecond })
	d.waitConnect()
	d.waitRooms(wire.TypeJoinRoom, helloRooms()...)
	d.push(frame(t, "game"))
	waitFor(t, "game cached", func() bool { return c.Mirror().Frame("smoke1", wire.ClassGame) != nil })

	d.setReject(true)
	d.closeConn()
	waitFor(t, "stale", func() bool { return c.Status().Stale })
	st := c.Status()
	if st.Connected || len(c.Mirror().Instances()) != 0 || c.Mirror().JoinReplayForHostAll() != nil {
		t.Fatalf("after stale: Status=%+v instances=%v", st, c.Mirror().Instances())
	}
	if !logs.has("mirror cleared") || !logs.has("reconnect in") {
		t.Fatal("stale / backoff not logged")
	}

	d.setReject(false)
	d.waitConnect()
	d.waitRooms(wire.TypeJoinRoom, helloRooms()...)
	waitFor(t, "second connect eviction", func() bool { return hub.evictedPrefix("host:") == 2 })
	st = c.Status()
	if !st.Connected || st.Stale || st.Reconnects != 1 {
		t.Fatalf("after reconnect: %+v", st)
	}
	if got := c.Mirror().Instances(); !reflect.DeepEqual(got, []string{"smoke1", "smoke2"}) {
		t.Fatalf("Instances after reconnect = %v", got)
	}
	if c.Mirror().Frame("smoke1", wire.ClassGame) != nil {
		t.Fatal("stale clear did not survive the reconnect")
	}
}

func TestStreamShortDisconnectKeepsMirror(t *testing.T) {
	d := newFakeDaemon(t)
	c, _ := newTestClient(t, d, nil, nil) // staleAfter 30s
	d.waitConnect()
	d.waitRooms(wire.TypeJoinRoom, helloRooms()...)
	d.push(frame(t, "game"))
	waitFor(t, "game cached", func() bool { return c.Mirror().Frame("smoke1", wire.ClassGame) != nil })

	d.closeConn()
	d.waitConnect()
	d.waitRooms(wire.TypeJoinRoom, helloRooms()...)
	if st := c.Status(); st.Stale || st.Reconnects != 1 || !st.Connected {
		t.Fatalf("Status = %+v", st)
	}
	if c.Mirror().Frame("smoke1", wire.ClassGame) == nil {
		t.Fatal("short disconnect dropped the cache")
	}
}

func TestStreamShedsFirehoseUnderBackpressure(t *testing.T) {
	d := newFakeDaemon(t)
	gate := make(chan struct{})
	var once sync.Once
	rec := &frameRecorder{}
	c, _ := newTestClient(t, d, nil, func(c *Client) {
		c.frameBuf = 8
		c.cfg.OnFrame = func(f Frame) {
			rec.OnFrame(f)
			if f.Env != nil && f.Env.Type == wire.ClassGame {
				<-gate // stall the worker on the first game frame
			}
		}
	})
	d.waitConnect()
	d.waitRooms(wire.TypeJoinRoom, helloRooms()...)

	d.push(frame(t, "game"))
	waitFor(t, "worker stalled", func() bool { _, ok := rec.find(wire.TypeScraper, wire.ClassGame); return ok })
	tick := frame(t, "tick")
	for i := 0; i < 40; i++ {
		d.push(tick)
	}
	// Every tick frame was either queued or shed once the pump has read all 40.
	waitFor(t, "pump drained the socket", func() bool { return c.Status().Shed+uint64(len(c.frames)) == 40 })
	d.push(frame(t, "scenario"))
	once.Do(func() { close(gate) })
	waitFor(t, "scenario delivered", func() bool { _, ok := rec.find(wire.TypeScraper, wire.ClassScenario); return ok })
	// Firehose frames are shed past 75% of an 8-deep buffer: 7 queued, 33 shed.
	if st := c.Status(); st.Shed != 33 {
		t.Fatalf("Shed = %d, want 33", st.Shed)
	}
	if c.Mirror().Frame("smoke1", wire.ClassTick) == nil {
		t.Fatal("no tick frame survived the shed")
	}
}

func TestStreamJoinLeaveDemand(t *testing.T) {
	d := newFakeDaemon(t)
	c, _ := newTestClient(t, d, nil, nil)
	d.waitConnect()
	d.waitRooms(wire.TypeJoinRoom, helloRooms()...)

	c.Join("host:smoke1:tick")
	d.waitRooms(wire.TypeJoinRoom, "host:smoke1:tick")
	c.Leave("host:smoke1:game") // always-class: never left upstream
	c.Leave("host:summary")
	c.Leave("host:smoke1:tick")
	seen := d.waitRooms(wire.TypeLeaveRoom, "host:smoke1:tick")
	if len(seen) != 1 {
		t.Fatalf("leave traffic = %v", seen)
	}
	c.Join("host:smoke2:objects")
	d.waitRooms(wire.TypeJoinRoom, "host:smoke2:objects")

	// Demand survives a reconnect.
	d.closeConn()
	d.waitConnect()
	d.waitRooms(wire.TypeJoinRoom, helloRooms("host:smoke2:objects")...)
	d.expectNoMsg(30 * time.Millisecond)
}

func TestStreamHelloStartedAtChangeDropsCache(t *testing.T) {
	d := newFakeDaemon(t)
	hub := &fakeHub{}
	c, logs := newTestClient(t, d, hub, nil)
	d.waitConnect()
	d.waitRooms(wire.TypeJoinRoom, helloRooms()...)
	d.push(frame(t, "game"))
	d.push(frameEnv(t, mutated(t, "game", map[string]any{"instance": "smoke2"})))
	waitFor(t, "both cached", func() bool {
		return c.Mirror().Frame("smoke1", wire.ClassGame) != nil && c.Mirror().Frame("smoke2", wire.ClassGame) != nil
	})

	var hp map[string]any
	if err := json.Unmarshal(fixture(t, "hello"), &hp); err != nil {
		t.Fatal(err)
	}
	data := hp["data"].(map[string]any)
	data["instances"] = []map[string]any{{"name": "smoke1", "started_at": "2026-09-01T12:30:00Z"}}
	d.setHello(frameEnv(t, mutated(t, "hello", map[string]any{"data": data})))
	d.closeConn()
	d.waitConnect()
	d.waitRooms(wire.TypeJoinRoom, alwaysRooms("smoke1")...)
	if c.Mirror().Frame("smoke1", wire.ClassGame) != nil {
		t.Fatal("started_at change kept smoke1's cache")
	}
	if got := c.Mirror().Instances(); !reflect.DeepEqual(got, []string{"smoke1"}) {
		t.Fatalf("Instances = %v; smoke2 left the hello", got)
	}
	if !logs.has("smoke1 started_at changed; cache dropped") {
		t.Fatal("epoch not logged")
	}
	waitFor(t, "second connect eviction", func() bool { return hub.evictedPrefix("host:") == 2 })
}
