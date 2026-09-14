package xcclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xemu-cartographer/xc-scraper/wire"
)

// TestStreamReconnectLogRedactsToken pins the feed token out of the
// reconnect log line: the dial error quotes the stream URL (?token=…).
func TestStreamReconnectLogRedactsToken(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // refused dial → *url.Error with the full request URL
	const secret = "SECRET FEED/TOKEN"
	logs := &logRecorder{}
	c, err := New(Config{URL: url, Token: secret, Log: logs.Logf})
	if err != nil {
		t.Fatal(err)
	}
	c.backoffMin, c.backoffMax = 20*time.Millisecond, 40*time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = c.Run(ctx) }()
	waitFor(t, "reconnect log", func() bool { return logs.has("reconnect in") })
	logs.mu.Lock()
	defer logs.mu.Unlock()
	for _, line := range logs.lines {
		if strings.Contains(line, secret) || strings.Contains(line, "SECRET+FEED") || strings.Contains(line, "SECRET%20FEED") {
			t.Fatalf("token leaked into the log: %q", line)
		}
		if strings.Contains(line, "reconnect in") && !strings.Contains(line, "token=***") {
			t.Fatalf("expected the redaction marker in %q", line)
		}
	}
}

// TestStreamStaleInversionKeepsCache pins the wire.md "About seq" reading: a
// lower seq whose started_at did not move is a stale frame, not a restart —
// nobody is evicted, the cache stands, the frame is still handed on.
func TestStreamStaleInversionKeepsCache(t *testing.T) {
	d := newFakeDaemon(t)
	hub := &fakeHub{}
	rec := &frameRecorder{}
	c, logs := newTestClient(t, d, hub, func(c *Client) { c.cfg.OnFrame = rec.OnFrame })
	d.waitConnect()
	d.waitRooms(wire.TypeJoinRoom, helloRooms()...)
	m := c.Mirror()

	d.push(frame(t, "scenario"))
	d.push(gameSeq(t, 41))
	waitFor(t, "game cached", func() bool { return m.Frame("smoke1", wire.ClassGame) != nil })
	at, _ := m.StartedAt("smoke1")
	d.setRows(http.StatusOK, instanceRow{Name: "smoke1", StartedAt: at})
	kept := m.Frame("smoke1", wire.ClassGame)

	d.push(gameSeq(t, 2))
	waitFor(t, "stale drop logged", func() bool { return logs.has("seq regression on game; started_at unchanged, stale frame dropped") })
	if d.fetchCount() != 1 {
		t.Fatalf("started_at confirmed via %d fetch(es), want 1", d.fetchCount())
	}
	if hub.evictedPrefix("host:smoke1") != 0 {
		t.Fatal("a stale frame must not evict")
	}
	if got := m.Frame("smoke1", wire.ClassGame); string(got) != string(kept) {
		t.Fatal("stale frame replaced the cached game frame")
	}
	if m.Frame("smoke1", wire.ClassScenario) == nil {
		t.Fatal("stale frame cleared the scenario frame")
	}
	waitFor(t, "stale frame relayed", func() bool {
		rec.mu.Lock()
		defer rec.mu.Unlock()
		n := 0
		for _, f := range rec.frames {
			if f.Env != nil && f.Env.Type == wire.ClassGame {
				n++
			}
		}
		return n == 2
	})
	// A fresh frame after the stale one is contiguous with the kept seq.
	d.push(gameSeq(t, 42))
	waitFor(t, "next frame cached", func() bool {
		raw := m.Frame("smoke1", wire.ClassGame)
		if raw == nil {
			return false
		}
		_, payload := payloadOf(t, raw)
		var env wire.Envelope
		_ = json.Unmarshal(payload, &env)
		return env.Seq == 42
	})
	if c.Status().SeqGaps != 0 {
		t.Fatalf("SeqGaps = %d after a stale frame", c.Status().SeqGaps)
	}
}

// TestStreamUnconfirmedInversionIsEpoch: when started_at cannot be read the
// regression is treated as a restart (the old behaviour) rather than
// freezing a genuinely restarted instance behind a stale cache.
func TestStreamUnconfirmedInversionIsEpoch(t *testing.T) {
	d := newFakeDaemon(t)
	hub := &fakeHub{}
	c, logs := newTestClient(t, d, hub, nil)
	d.waitConnect()
	d.waitRooms(wire.TypeJoinRoom, helloRooms()...)
	m := c.Mirror()
	d.push(frame(t, "scenario"))
	d.push(gameSeq(t, 41))
	waitFor(t, "game cached", func() bool { return m.Frame("smoke1", wire.ClassGame) != nil })
	d.setRows(http.StatusInternalServerError)
	d.push(gameSeq(t, 2))
	waitFor(t, "epoch eviction", func() bool { return hub.evictedPrefix("host:smoke1") == 1 })
	if !logs.has("epoch change (started_at unconfirmed)") {
		t.Fatal("unconfirmed epoch not logged")
	}
	if m.Frame("smoke1", wire.ClassScenario) != nil || m.Frame("smoke1", wire.ClassGame) == nil {
		t.Fatal("unconfirmed epoch must clear the cache and keep the new frame")
	}
	if !m.Placeholder("smoke1") {
		t.Fatal("unconfirmed epoch must leave a placeholder for the next started_at refresh")
	}
}

// TestDemandRefcountWithSink pins the shared demand set: a room the pb: event
// writer holds (Client.Join) survives the demand layer's linger leave, its
// cached frame included, and is left upstream only when the last owner goes.
func TestDemandRefcountWithSink(t *testing.T) {
	d := newFakeDaemon(t)
	c, logs := newTestClient(t, d, nil, nil)
	d.waitConnect()
	d.waitRooms(wire.TypeJoinRoom, helloRooms()...)
	const room = "host:smoke1:" + wire.ClassTick

	c.Join(room) // the sink's reference
	d.waitRooms(wire.TypeJoinRoom, room)
	dm := NewDemand(c, 20*time.Millisecond)
	defer dm.Close()
	dm.Observe(room, true) // 0→1 downstream: second reference, no second join_room
	d.expectNoMsg(30 * time.Millisecond)
	d.push(frame(t, "tick"))
	waitFor(t, "tick cached", func() bool { return c.Mirror().Frame("smoke1", wire.ClassTick) != nil })

	dm.Observe(room, false)
	waitFor(t, "linger release", func() bool { return logs.has("still wanted by a sink") })
	d.expectNoMsg(30 * time.Millisecond) // no leave_room: the sink still wants it
	if !wantedRooms(c)[room] || c.Mirror().Frame("smoke1", wire.ClassTick) == nil {
		t.Fatal("sink-held room or its frame dropped by the demand release")
	}
	if len(dm.Rooms()) != 0 {
		t.Fatalf("demand rooms after release = %v", dm.Rooms())
	}

	c.Leave(room) // last owner
	d.waitRooms(wire.TypeLeaveRoom, room)
	if wantedRooms(c)[room] {
		t.Fatal("room still wanted after the last Leave")
	}
	// Balanced ownership the other way round: demand first, sink second.
	dm.Observe(room, true)
	d.waitRooms(wire.TypeJoinRoom, room)
	c.Join(room)
	c.Leave(room)
	d.expectNoMsg(30 * time.Millisecond)
	if !wantedRooms(c)[room] {
		t.Fatal("sink Join/Leave dropped the demand layer's reference")
	}
}

// TestFinishedGameDetector pins the D-7 line: a previous_game frame naming a
// finished game that is still not persisted after the grace logs once.
func TestFinishedGameDetector(t *testing.T) {
	var mu sync.Mutex
	exists := map[string]bool{"persisted": true}
	logs := &logRecorder{}
	var fired []func()
	det := NewFinishedGameDetector(func(uid string) bool {
		mu.Lock()
		defer mu.Unlock()
		return exists[uid]
	}, logs.Logf)
	det.timer = func(d time.Duration, fn func()) {
		if d != FinishedGameGrace {
			t.Fatalf("grace = %s", d)
		}
		fired = append(fired, fn)
	}
	prev := func(uid string) Frame {
		var env wire.Envelope
		if err := json.Unmarshal(fixture(t, "previous_game"), &env); err != nil {
			t.Fatal(err)
		}
		var data map[string]any
		if err := json.Unmarshal(env.Data, &data); err != nil {
			t.Fatal(err)
		}
		if uid == "" {
			delete(data, "finished_game")
		} else {
			data["finished_game"] = map[string]any{"game_uid": uid}
		}
		env.Data, _ = json.Marshal(data)
		return Frame{Env: &env}
	}
	det.OnFrame(prev("missing"))
	det.OnFrame(prev("missing")) // replayed on every join: armed once
	det.OnFrame(prev("persisted"))
	det.OnFrame(Frame{Raw: frame(t, "game"), Env: &wire.Envelope{Type: wire.ClassGame, Data: json.RawMessage(`{"finished_game":{"game_uid":"not-a-previous-game"}}`)}})
	det.OnFrame(prev(""))
	if det.Seen() != 2 || len(fired) != 2 {
		t.Fatalf("armed %d / seen %d, want 2 / 2", len(fired), det.Seen())
	}
	for _, fn := range fired {
		fn()
	}
	if !logs.has("xcclient: finished_game missing seen on stream, not persisted after 30s") {
		t.Fatal("D-7 line missing")
	}
	if logs.has("finished_game persisted") {
		t.Fatal("persisted game logged")
	}
	// nil deps disable the detector.
	NewFinishedGameDetector(nil, nil).OnFrame(prev("x"))
	var nilDet *FinishedGameDetector
	nilDet.OnFrame(prev("y"))
}
