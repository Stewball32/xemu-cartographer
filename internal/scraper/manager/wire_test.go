package manager

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/guards"
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/capture"
	"github.com/Stewball32/xemu-cartographer/internal/websocket"
)

// newTestRunner builds a minimal *runner suitable for exercising the
// per-class envelope builders without standing up a real xemu instance
// or aggregator. Tests prepare the cache directly under r.cacheMu (or
// via withCache), then call the v2 build/broadcast methods.
func newTestRunner(name string) *runner {
	return newRunner(name, "/tmp/sock", "host:"+name, nil, nil)
}

// decodeClassEnvelope unwraps a marshaled websocket.Message to the inner
// scraper.Envelope. Used by per-class assertions.
func decodeClassEnvelope(t *testing.T, data []byte) (websocket.Message, scraper.Envelope) {
	t.Helper()
	var msg websocket.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal websocket.Message: %v", err)
	}
	var env scraper.Envelope
	if err := json.Unmarshal(msg.Payload, &env); err != nil {
		t.Fatalf("unmarshal scraper.Envelope: %v", err)
	}
	return msg, env
}

// TestClassEnvelopeMessagesIdle: a bare cache (no game data, no tick)
// produces just the always-present classes — xbox and game. The
// optional classes (scenario, tick, objects, debug, previous_game)
// return nil from their adapters and are skipped.
func TestClassEnvelopeMessagesIdle(t *testing.T) {
	r := newTestRunner("alpha")
	defer r.cancel()
	r.withCache(func(c *instanceCache) {
		c.Phase = PhaseIdle
		c.TitleID = 0x9E115330
	})

	msgs := r.classEnvelopeMessages()
	got := map[string]bool{}
	for _, m := range msgs {
		got[m.Class] = true
	}
	if !got["xbox"] || !got["game"] {
		t.Fatalf("idle: want xbox + game, got %v", got)
	}
	for _, optional := range []string{"scenario", "tick", "objects", "debug", "previous_game"} {
		if got[optional] {
			t.Fatalf("idle: %q should not be present, got %v", optional, got)
		}
	}
}

// TestClassEnvelopeMessagesLive: populated cache produces xbox +
// scenario + game + tick + objects + debug. previous_game stays absent
// (no Live → Ready capture yet).
func TestClassEnvelopeMessagesLive(t *testing.T) {
	r := newTestRunner("bravo")
	defer r.cancel()
	r.withCache(func(c *instanceCache) {
		c.Phase = PhaseLive
		c.TitleID = 0x4D530004
		c.Title = "Halo: Combat Evolved"
		c.EngineTick = 1234
		c.GameData = &scraper.GameData{Map: "bloodgulch", Gametype: "slayer"}
		c.LatestTick = &scraper.TickPayload{
			PlayerCount: 2,
			Players: []scraper.TickPlayer{
				{Index: 0, Alive: true, X: 1, Y: 2, Z: 3},
			},
		}
	})

	msgs := r.classEnvelopeMessages()
	got := map[string]bool{}
	for _, m := range msgs {
		got[m.Class] = true
	}
	for _, want := range []string{"xbox", "scenario", "game", "tick", "objects", "debug"} {
		if !got[want] {
			t.Fatalf("live: missing class %q (got %v)", want, got)
		}
	}
	if got["previous_game"] {
		t.Fatalf("live: previous_game should be absent until Live→Ready, got %v", got)
	}

	// Spot-check one envelope: the tick envelope should have Type="tick"
	// and route to host:bravo:tick.
	for _, m := range msgs {
		if m.Class != "tick" {
			continue
		}
		msg, env := decodeClassEnvelope(t, m.Bytes)
		if msg.Room != "host:bravo:tick" {
			t.Fatalf("tick room = %q, want %q", msg.Room, "host:bravo:tick")
		}
		if env.Type != "tick" {
			t.Fatalf("tick envelope type = %q, want %q", env.Type, "tick")
		}
		if env.V != scraper.ProtocolVersion {
			t.Fatalf("tick envelope v = %d, want %d", env.V, scraper.ProtocolVersion)
		}
		if env.Instance != "bravo" {
			t.Fatalf("tick envelope instance = %q, want %q", env.Instance, "bravo")
		}
	}
}

// TestClassEnvelopeMessagesWithPreviousGame: previous_game appears when
// the cache holds a captured previous game (Live → Ready transition).
func TestClassEnvelopeMessagesWithPreviousGame(t *testing.T) {
	r := newTestRunner("charlie")
	defer r.cancel()
	r.withCache(func(c *instanceCache) {
		c.Phase = PhaseReady
		c.GameData = &scraper.GameData{Map: "wizard"}
		c.PreviousGame = &previousGame{
			GameData: &scraper.GameData{Map: "wizard"},
			EndedAt:  time.Now(),
		}
	})

	got := map[string]bool{}
	for _, m := range r.classEnvelopeMessages() {
		got[m.Class] = true
	}
	if !got["previous_game"] {
		t.Fatalf("ready w/ previous_game: missing previous_game class, got %v", got)
	}
}

// TestBroadcastSnapshotEmitsToPerClassRooms: broadcastSnapshot fans out
// every applicable class to its host:<inst>:<class> room.
func TestBroadcastSnapshotEmitsToPerClassRooms(t *testing.T) {
	ws := &stubWS{}
	svc := &guards.Services{WS: ws}
	r := newTestRunner("alpha")
	defer r.cancel()
	r.withCache(func(c *instanceCache) {
		c.Phase = PhaseLive
		c.GameData = &scraper.GameData{Map: "bloodgulch"}
		c.LatestTick = &scraper.TickPayload{}
	})

	r.broadcastSnapshot(svc)

	sends := ws.snapshot()
	if len(sends) == 0 {
		t.Fatal("broadcastSnapshot: no WS sends recorded")
	}
	wantPrefixes := []string{
		"host:alpha:xbox",
		"host:alpha:scenario",
		"host:alpha:game",
		"host:alpha:tick",
		"host:alpha:objects",
		"host:alpha:debug",
	}
	got := map[string]bool{}
	for _, s := range sends {
		got[s.Room] = true
	}
	for _, want := range wantPrefixes {
		if !got[want] {
			t.Fatalf("broadcastSnapshot: missing room %q (got %v)", want, got)
		}
	}
}

// TestBroadcastPollGameAndTick: broadcastPoll emits game when LatestTick
// is unset (Idle case) and adds tick / objects / debug when it's set.
// PR 15: broadcastPoll is now demand-gated, so the per-class rooms must
// be marked occupied for the emit to happen — that mirrors the live
// behaviour where the broadcast skips classes with no subscribers.
func TestBroadcastPollGameAndTick(t *testing.T) {
	ws := &stubWS{occupied: map[string]bool{"host:alpha:game": true}}
	svc := &guards.Services{WS: ws}
	r := newTestRunner("alpha")
	defer r.cancel()

	// Idle / no tick — only `game` should emit.
	r.withCache(func(c *instanceCache) { c.Phase = PhaseIdle })
	r.broadcastPoll(svc)
	sends := ws.snapshot()
	if len(sends) != 1 || sends[0].Room != "host:alpha:game" {
		t.Fatalf("poll w/o tick: want one send to host:alpha:game, got %+v", sends)
	}

	// Now add a tick — game + tick + objects + debug all emit (every
	// room marked occupied).
	ws2 := &stubWS{occupied: map[string]bool{
		"host:alpha:game":    true,
		"host:alpha:tick":    true,
		"host:alpha:objects": true,
		"host:alpha:debug":   true,
	}}
	svc2 := &guards.Services{WS: ws2}
	r.withCache(func(c *instanceCache) {
		c.Phase = PhaseLive
		c.LatestTick = &scraper.TickPayload{}
	})
	r.broadcastPoll(svc2)
	got := map[string]bool{}
	for _, s := range ws2.snapshot() {
		got[s.Room] = true
	}
	for _, want := range []string{"host:alpha:game", "host:alpha:tick", "host:alpha:objects", "host:alpha:debug"} {
		if !got[want] {
			t.Fatalf("poll w/ tick: missing %q (got %v)", want, got)
		}
	}
}

// TestBroadcastPollSkipsUnsubscribedClasses: broadcastPoll must respect
// per-class demand — with only host:alpha:debug occupied, the runner
// emits debug but nothing else, even when LatestTick is set.
func TestBroadcastPollSkipsUnsubscribedClasses(t *testing.T) {
	ws := &stubWS{occupied: map[string]bool{"host:alpha:debug": true}}
	svc := &guards.Services{WS: ws}
	r := newTestRunner("alpha")
	defer r.cancel()

	r.withCache(func(c *instanceCache) {
		c.Phase = PhaseLive
		c.LatestTick = &scraper.TickPayload{}
	})
	r.broadcastPoll(svc)

	sends := ws.snapshot()
	if len(sends) != 1 || sends[0].Room != "host:alpha:debug" {
		t.Fatalf("partial demand: want one send to host:alpha:debug, got %+v", sends)
	}
}

// TestBroadcastPollAlwaysModeBypassesWS: capture policy `always` for a
// class forces the broadcast even with the WS room empty. This is the
// "record without viewers" case — the runner must still hand the marshal
// to the WS hub (which no-ops on empty rooms) so the same envelope can
// later be teed to a sink (PR 17+).
func TestBroadcastPollAlwaysModeBypassesWS(t *testing.T) {
	ws := &stubWS{occupied: map[string]bool{}}
	svc := &guards.Services{WS: ws}
	r := newTestRunner("alpha")
	defer r.cancel()
	r.setPolicies([]capture.Policy{
		{Instance: "alpha", Class: "tick", Mode: capture.ModeAlways},
	})

	r.withCache(func(c *instanceCache) {
		c.Phase = PhaseLive
		c.LatestTick = &scraper.TickPayload{}
	})
	r.broadcastPoll(svc)

	sends := ws.snapshot()
	if len(sends) != 1 || sends[0].Room != "host:alpha:tick" {
		t.Fatalf("always policy: want one send to host:alpha:tick, got %+v", sends)
	}
}

// TestBroadcastRoutesByClass: the generic broadcast() (used by the
// haloce events package) routes by envelope.Type — an "event" envelope
// lands on host:<inst>:event.
func TestBroadcastRoutesByClass(t *testing.T) {
	ws := &stubWS{}
	svc := &guards.Services{WS: ws}
	r := newTestRunner("alpha")
	defer r.cancel()

	env := scraper.MakeEnvelope("event", "alpha", 1, 42, map[string]any{"event_type": "death"})
	r.broadcast(svc, env)

	sends := ws.snapshot()
	if len(sends) != 1 {
		t.Fatalf("broadcast: want one send, got %d", len(sends))
	}
	if sends[0].Room != "host:alpha:event" {
		t.Fatalf("broadcast: routed to %q, want %q", sends[0].Room, "host:alpha:event")
	}
}
