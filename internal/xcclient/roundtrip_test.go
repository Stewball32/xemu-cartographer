package xcclient

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/xemu-cartographer/xc-scraper/hub"
	"github.com/xemu-cartographer/xc-scraper/wire"
)

// wantedRooms snapshots the client's demand set.
func wantedRooms(c *Client) map[string]bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]bool, len(c.wanted))
	for k, v := range c.wanted {
		out[k] = v > 0
	}
	return out
}

// payloadOf decodes the outer message of a frame and returns its payload.
func payloadOf(t *testing.T, raw []byte) (wire.Message, []byte) {
	t.Helper()
	var msg wire.Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("decode frame: %v", err)
	}
	return msg, []byte(msg.Payload)
}

// expectResync asserts every eviction on prefix is the 1012 "upstream
// resync" close (no error frame: the status/reason are what EvictRoomPrefix
// turns into Close(1012, reason), F2).
func expectResync(t *testing.T, sink *sinkHub, prefix string) {
	t.Helper()
	n := 0
	for _, e := range sink.evicted() {
		if e.Prefix != prefix {
			continue
		}
		n++
		if e.Status != websocket.StatusServiceRestart || e.Reason != EvictReason {
			t.Fatalf("eviction on %s = %d %q, want 1012 %q", prefix, e.Status, e.Reason, EvictReason)
		}
	}
	if n == 0 {
		t.Fatalf("no eviction on %s (got %v)", prefix, sink.evicted())
	}
}

// TestRoundTripHub drives a real xc/hub (fixture Replayer) → xcclient →
// HubPort: hello → joins → replay bytes == fixture → rebroadcast room/bytes
// preserved → occupancy hook joins/leaves upstream → epoch change evicts
// 1012 → stale clear.
func TestRoundTripHub(t *testing.T) {
	rp := newFixtureReplayer(t, "smoke1", "smoke2")
	gameEnv := rp.serve(t, "game")
	tickEnv := rp.serve(t, "tick")
	summaryEnv := rp.serve(t, "summary")
	rp.serve(t, "scenario")
	up := newUpstream(t, rp)

	sink := &sinkHub{}
	rb := NewRebroadcaster(sink, nil)
	c, logs := bootClient(t, up.url, func(cfg *Config) {
		cfg.Hub = sink
		cfg.OnFrame = rb.OnFrame
		cfg.Hostrunner = true
		cfg.StaleAfter = 200 * time.Millisecond
	}, nil)

	// hello → joins → replay.
	gameRoom := wire.HostRoomPrefix + ":smoke1:" + wire.ClassGame
	waitFor(t, "hello + replay", func() bool {
		return c.Status().Connected && c.Mirror().Frame("smoke1", wire.ClassGame) != nil && len(sink.sent(wire.SummaryRoom)) > 0
	})
	for _, room := range []string{wire.SummaryRoom, gameRoom, wire.HostRoomPrefix + ":smoke2:" + wire.ClassXbox, HostrunnerRoom} {
		waitFor(t, "upstream membership "+room, func() bool { return up.hub.RoomHasMembers(room) })
	}
	if got, want := c.Mirror().Frame("smoke1", wire.ClassGame), hub.Frame(gameRoom, gameEnv); !bytes.Equal(got, want) {
		t.Fatalf("mirror game frame\n got %s\nwant %s", got, want)
	}
	_, payload := payloadOf(t, c.Mirror().Frame("smoke1", wire.ClassGame))
	if !bytes.Equal(payload, gameEnv) {
		t.Fatalf("replayed envelope != fixture:\n got %s\nwant %s", payload, gameEnv)
	}
	if got, ok := c.Mirror().Summary(); !ok || len(got.Hosts) != 2 {
		t.Fatalf("summary not mirrored: %+v %v", got, ok)
	}

	// rebroadcast: same room, same bytes; hello never forwarded.
	if got := sink.sent(gameRoom); len(got) != 1 || !bytes.Equal(got[0], hub.Frame(gameRoom, gameEnv)) {
		t.Fatalf("rebroadcast on %s = %d frame(s) %q", gameRoom, len(got), got)
	}
	if got := sink.sent(wire.SummaryRoom); len(got) != 1 || !bytes.Equal(got[0], hub.Frame(wire.SummaryRoom, summaryEnv)) {
		t.Fatalf("rebroadcast on %s = %d frame(s)", wire.SummaryRoom, len(got))
	}
	if rooms := sink.rooms(); rooms[""] != 0 || rooms[AdminRoom] != 0 {
		t.Fatalf("unexpected rebroadcast rooms: %v", rooms)
	}
	if rb.Dropped() == 0 {
		t.Fatal("hello should count as dropped")
	}
	expectResync(t, sink, wire.HostRoomPrefix+":")

	// host_runner → admin room, re-framed.
	hrPayload := json.RawMessage(`{"kind":"selection","map":"bloodgulch"}`)
	hrUp, _ := json.Marshal(wire.Message{Type: HostRunnerType, Room: HostrunnerRoom, Payload: hrPayload})
	up.hub.SendToRoomRaw(HostrunnerRoom, hrUp)
	waitFor(t, "host_runner rebroadcast", func() bool { return len(sink.sent(AdminRoom)) == 1 })
	wantHR, _ := json.Marshal(wire.Message{Type: HostRunnerType, Room: AdminRoom, Payload: hrPayload})
	if got := sink.sent(AdminRoom)[0]; !bytes.Equal(got, wantHR) {
		t.Fatalf("admin frame\n got %s\nwant %s", got, wantHR)
	}

	// occupancy hook: 0→1 joins upstream (replay flows), 1→0 leaves after
	// the linger and drops the cached frame; re-occupancy inside the linger
	// cancels the leave; always-rooms never leave.
	const linger = 60 * time.Millisecond
	d := NewDemand(c, linger)
	defer d.Close()
	tickRoom := wire.HostRoomPrefix + ":smoke1:" + wire.ClassTick
	d.Observe(tickRoom, true)
	waitFor(t, "demand join upstream", func() bool { return up.hub.RoomHasMembers(tickRoom) })
	waitFor(t, "tick replay", func() bool { return c.Mirror().Frame("smoke1", wire.ClassTick) != nil })
	if got := sink.sent(tickRoom); len(got) != 1 || !bytes.Equal(got[0], hub.Frame(tickRoom, tickEnv)) {
		t.Fatalf("tick rebroadcast = %d frame(s)", len(got))
	}
	if got := d.Rooms(); len(got) != 1 || got[0] != tickRoom {
		t.Fatalf("Rooms = %v", got)
	}
	d.Observe(tickRoom, false)
	if !d.Lingering(tickRoom) {
		t.Fatal("expected linger after 1→0")
	}
	d.Observe(tickRoom, true)
	time.Sleep(3 * linger)
	if !up.hub.RoomHasMembers(tickRoom) || d.Lingering(tickRoom) {
		t.Fatal("re-occupancy inside the linger must cancel the leave")
	}
	d.Observe(tickRoom, false)
	waitFor(t, "demand leave upstream", func() bool { return !up.hub.RoomHasMembers(tickRoom) })
	waitFor(t, "cached tick dropped", func() bool { return c.Mirror().Frame("smoke1", wire.ClassTick) == nil })
	if len(d.Rooms()) != 0 || wantedRooms(c)[tickRoom] {
		t.Fatalf("demand set after leave: %v %v", d.Rooms(), wantedRooms(c))
	}
	d.Observe(gameRoom, false)
	d.Observe(wire.SummaryRoom, false)
	d.Observe("admin", true)
	time.Sleep(3 * linger)
	if !up.hub.RoomHasMembers(gameRoom) || !up.hub.RoomHasMembers(wire.SummaryRoom) || wantedRooms(c)["admin"] {
		t.Fatal("always/non-host rooms must not be demand-managed")
	}
	if c.Mirror().Frame("smoke1", wire.ClassGame) == nil {
		t.Fatal("always-class frame must survive a 1→0 on its room")
	}

	// seq inversion with the same started_at (join replay racing a
	// broadcast, wire.md "About seq"): the stale frame is dropped from the
	// mirror, relayed downstream as-is, and nobody is evicted.
	before := sink.evictedPrefix(wire.HostRoomPrefix + ":smoke1")
	kept := c.Mirror().Frame("smoke1", wire.ClassGame)
	up.hub.SendToRoomRaw(gameRoom, hub.Frame(gameRoom, mutated(t, "game", map[string]any{"seq": 40})))
	waitFor(t, "stale frame logged", func() bool { return logs.has("stale frame dropped") })
	if got := sink.sent(gameRoom); len(got) != 2 {
		t.Fatalf("stale frame still rebroadcast: %d frame(s)", len(got))
	}
	if sink.evictedPrefix(wire.HostRoomPrefix+":smoke1") != before {
		t.Fatal("same started_at must not evict")
	}
	if got := c.Mirror().Frame("smoke1", wire.ClassGame); !bytes.Equal(got, kept) {
		t.Fatal("stale frame replaced the cached game frame")
	}

	// epoch change: a seq regression whose started_at moved evicts
	// host:<inst> with 1012 only.
	rp.restart(time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC))
	up.hub.SendToRoomRaw(gameRoom, hub.Frame(gameRoom, mutated(t, "game", map[string]any{"seq": 39})))
	waitFor(t, "epoch eviction", func() bool { return sink.evictedPrefix(wire.HostRoomPrefix+":smoke1") > before })
	expectResync(t, sink, wire.HostRoomPrefix+":smoke1")
	if got := sink.sent(gameRoom); len(got) != 3 {
		t.Fatalf("epoch frame still rebroadcast: %d frame(s)", len(got))
	}
	if !logs.has("seq regression on game; epoch change") {
		t.Fatal("expected a seq regression epoch log line")
	}
	if got := c.Mirror().Frame("smoke1", wire.ClassGame); bytes.Equal(got, kept) {
		t.Fatal("epoch frame must replace the cached game frame")
	}

	// stale clear: upstream gone longer than StaleAfter empties the mirror.
	up.stop()
	waitFor(t, "stale clear", func() bool { return c.Status().Stale && len(c.Mirror().Instances()) == 0 })
	if c.Status().Connected {
		t.Fatal("still connected after upstream stop")
	}
}

// TestRoundTripDaemon boots the real xc/daemon (AllowEmpty) and checks the
// consumer path end to end: token hello, summary-driven instance add, frame
// rebroadcast byte-for-byte, demand join/leave visible on the daemon hub,
// stale clear on shutdown.
func TestRoundTripDaemon(t *testing.T) {
	base, s, stop := bootDaemon(t)
	sink := &sinkHub{}
	rb := NewRebroadcaster(sink, nil)
	c, _ := bootClient(t, base, func(cfg *Config) {
		cfg.Hub = sink
		cfg.OnFrame = rb.OnFrame
		cfg.StaleAfter = 200 * time.Millisecond
	}, nil)
	waitFor(t, "connect", func() bool { return c.Status().Connected })
	waitFor(t, "summary join", func() bool { return s.Hub().RoomHasMembers(wire.SummaryRoom) })
	if len(c.Mirror().Instances()) != 0 {
		t.Fatalf("empty daemon announced instances: %v", c.Mirror().Instances())
	}

	s.Hub().SendToRoomRaw(wire.SummaryRoom, summaryFor(t, "2026-09-01T12:00:00Z", "smoke1"))
	gameRoom := wire.HostRoomPrefix + ":smoke1:" + wire.ClassGame
	waitFor(t, "instance added + always rooms joined", func() bool {
		return len(c.Mirror().Instances()) == 1 && s.Hub().RoomHasMembers(gameRoom)
	})
	gameEnv := fixture(t, "game")
	s.Hub().SendToRoomRaw(gameRoom, hub.Frame(gameRoom, gameEnv))
	waitFor(t, "game rebroadcast", func() bool { return len(sink.sent(gameRoom)) == 1 })
	if got := sink.sent(gameRoom)[0]; !bytes.Equal(got, hub.Frame(gameRoom, gameEnv)) {
		t.Fatalf("daemon frame altered:\n got %s\nwant %s", got, hub.Frame(gameRoom, gameEnv))
	}
	if list := c.Mirror().List(); len(list) != 1 || list[0].Name != "smoke1" {
		t.Fatalf("List = %+v", list)
	}

	d := NewDemand(c, 50*time.Millisecond)
	defer d.Close()
	tickRoom := wire.HostRoomPrefix + ":smoke1:" + wire.ClassTick
	d.Observe(tickRoom, true)
	waitFor(t, "demand join on daemon hub", func() bool { return s.Hub().RoomHasMembers(tickRoom) })
	d.Observe(tickRoom, false)
	waitFor(t, "demand leave on daemon hub", func() bool { return !s.Hub().RoomHasMembers(tickRoom) })
	if !s.Hub().RoomHasMembers(gameRoom) {
		t.Fatal("always room left with the demand room")
	}

	stop()
	waitFor(t, "stale clear", func() bool { return c.Status().Stale && len(c.Mirror().Instances()) == 0 })
}

// TestRebroadcastFilters pins the §8.4 filter without a network: reply-only
// classes, roomless frames, error/unknown types and a nil hub are dropped;
// host_runner is re-framed onto admin.
func TestRebroadcastFilters(t *testing.T) {
	sink := &sinkHub{}
	rb := NewRebroadcaster(sink, nil)
	scraper := func(room string, env []byte) Frame {
		var hdr wire.Envelope
		if err := json.Unmarshal(env, &hdr); err != nil {
			t.Fatal(err)
		}
		raw := hub.Frame(room, env)
		var msg wire.Message
		_ = json.Unmarshal(raw, &msg)
		return Frame{Raw: raw, Msg: msg, Env: &hdr}
	}
	gameRoom := wire.HostRoomPrefix + ":smoke1:" + wire.ClassGame
	rb.OnFrame(scraper(gameRoom, fixture(t, "game")))
	rb.OnFrame(scraper("", fixture(t, "hello")))
	rb.OnFrame(scraper(wire.HostRoomPrefix+":smoke1", fixture(t, "events")))
	rb.OnFrame(scraper("", fixture(t, "game")))
	rb.OnFrame(Frame{Raw: []byte(`{"type":"error","payload":{}}`), Msg: wire.Message{Type: wire.TypeError}})
	rb.OnFrame(Frame{Raw: []byte(`{"type":"mystery"}`), Msg: wire.Message{Type: "mystery"}})
	rb.OnFrame(Frame{Msg: wire.Message{Type: HostRunnerType, Room: HostrunnerRoom, Payload: json.RawMessage(`{"a":1}`)}})
	if rooms := sink.rooms(); len(rooms) != 2 || rooms[gameRoom] != 1 || rooms[AdminRoom] != 1 {
		t.Fatalf("rooms = %v", rooms)
	}
	if got, want := rb.Forwarded(), uint64(2); got != want {
		t.Fatalf("Forwarded = %d, want %d", got, want)
	}
	if got, want := rb.Dropped(), uint64(5); got != want {
		t.Fatalf("Dropped = %d, want %d", got, want)
	}
	msg, _ := payloadOf(t, sink.sent(AdminRoom)[0])
	if msg.Type != HostRunnerType || msg.Room != AdminRoom || string(msg.Payload) != `{"a":1}` {
		t.Fatalf("admin frame = %+v", msg)
	}

	none := NewRebroadcaster(nil, nil)
	none.OnFrame(scraper(gameRoom, fixture(t, "game")))
	if none.Forwarded() != 0 || none.Dropped() != 1 {
		t.Fatal("nil hub must drop")
	}
}

// TestDemandRoom pins which rooms the occupancy hook manages.
func TestDemandRoom(t *testing.T) {
	cases := map[string]bool{
		"host:smoke1:tick":           true,
		"host:smoke1:objects":        true,
		"host:smoke1:debug":          true,
		"host:smoke1:game_filtered":  true,
		"host:smoke1:event_filtered": true,
		"host:smoke1:game":           false,
		"host:smoke1:event":          false,
		"host:smoke1":                false,
		"host:all":                   false,
		"host:summary":               false,
		"host:smoke1:nope":           false,
		"admin":                      false,
		"xc:hostrunner":              false,
		"":                           false,
	}
	for room, want := range cases {
		if _, got := DemandRoom(room); got != want {
			t.Errorf("DemandRoom(%q) = %v, want %v", room, got, want)
		}
	}
}
