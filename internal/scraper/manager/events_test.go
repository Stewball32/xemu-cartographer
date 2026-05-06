package manager

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/websocket"
)

// makeEvent is a small helper for building events the cache stores. Mirrors
// the Halo: CE detector's emit shape: outer envelope Type is always the
// literal "event" wire-type, semantic type lives in payload.event_type.
// Test helpers must match runtime emit shape so filterEvents covers the
// same code path real events take.
func makeEvent(tick uint32, typ string) scraper.Envelope {
	return scraper.MakeEnvelope("event", "smoke", tick, map[string]any{
		"event_type": typ,
	})
}

// TestFilterEventsEmpty: empty input returns a non-nil empty slice so the
// caller can serialise a `[]` rather than `null` payload.
func TestFilterEventsEmpty(t *testing.T) {
	out := filterEvents(nil, 0, nil)
	if out == nil {
		t.Fatal("filterEvents(nil, ...) = nil, want []scraper.Envelope{}")
	}
	if len(out) != 0 {
		t.Fatalf("filterEvents(nil, ...) len = %d, want 0", len(out))
	}
}

// TestFilterEventsReversesToOldestFirst: cache is newest-first; output
// must be oldest-first per the M5 brief's OQ7 resolution.
func TestFilterEventsReversesToOldestFirst(t *testing.T) {
	// Cache order (newest-first): [tick=3, tick=2, tick=1].
	cache := []scraper.Envelope{
		makeEvent(3, scraper.EventKill),
		makeEvent(2, scraper.EventDeath),
		makeEvent(1, scraper.EventSpawn),
	}
	out := filterEvents(cache, 0, nil)
	if len(out) != 3 {
		t.Fatalf("len = %d, want 3", len(out))
	}
	if out[0].Tick != 1 || out[1].Tick != 2 || out[2].Tick != 3 {
		t.Fatalf("ticks out of order: got [%d, %d, %d], want [1, 2, 3]", out[0].Tick, out[1].Tick, out[2].Tick)
	}
}

// TestFilterEventsSinceTick: only events with tick > sinceTick survive.
func TestFilterEventsSinceTick(t *testing.T) {
	cache := []scraper.Envelope{
		makeEvent(5, scraper.EventKill),
		makeEvent(4, scraper.EventDeath),
		makeEvent(3, scraper.EventSpawn),
		makeEvent(2, scraper.EventKill),
	}
	out := filterEvents(cache, 3, nil)
	gotTicks := make([]uint32, len(out))
	for i, e := range out {
		gotTicks[i] = e.Tick
	}
	want := []uint32{4, 5} // oldest-first, > 3
	if !reflect.DeepEqual(gotTicks, want) {
		t.Fatalf("ticks = %v, want %v", gotTicks, want)
	}
}

// TestFilterEventsTypes: only events whose Type is in the requested set
// survive; nil/empty type filter means no filter.
func TestFilterEventsTypes(t *testing.T) {
	cache := []scraper.Envelope{
		makeEvent(4, scraper.EventKill),
		makeEvent(3, scraper.EventDeath),
		makeEvent(2, scraper.EventSpawn),
		makeEvent(1, scraper.EventKill),
	}
	out := filterEvents(cache, 0, []string{scraper.EventKill})
	if len(out) != 2 {
		t.Fatalf("len = %d, want 2 (only kill events)", len(out))
	}
	for _, e := range out {
		if got := eventInnerType(e); got != scraper.EventKill {
			t.Fatalf("inner event_type = %q, want %q", got, scraper.EventKill)
		}
	}
}

// TestEventsReplyUnknownInstance: missing runner returns (nil, false).
func TestEventsReplyUnknownInstance(t *testing.T) {
	m := New(nil)
	defer m.Close()

	bytes, ok := m.EventsReply("nope", 0, nil)
	if ok {
		t.Fatalf("EventsReply(unknown): want ok=false, got bytes=%q", string(bytes))
	}
}

// TestEventsReplyIdleReturnsEmpty: an Idle runner returns the events
// envelope with empty Events even when filters would otherwise match —
// the M5 brief's OQ1 resolution. Phase comes back so the client knows.
func TestEventsReplyIdleReturnsEmpty(t *testing.T) {
	m := New(nil)
	defer m.Close()

	r := newRunner("alpha", "/tmp/sock", "host:alpha", nil, nil)
	defer r.cancel()
	r.cache.Phase = PhaseIdle
	// Stash some events on the cache to prove they're filtered out by phase
	// rather than absence.
	r.cache.Events = []scraper.Envelope{makeEvent(2, scraper.EventKill)}
	m.runners["alpha"] = r

	bytes, ok := m.EventsReply("alpha", 0, nil)
	if !ok {
		t.Fatal("EventsReply: ok=false")
	}

	payload := decodeEventsReply(t, bytes)
	if payload.Phase != PhaseIdle {
		t.Fatalf("payload.phase = %q, want %q", payload.Phase, PhaseIdle)
	}
	if len(payload.Events) != 0 {
		t.Fatalf("payload.events len = %d, want 0 in Idle", len(payload.Events))
	}
}

// TestEventsReplyLiveReturnsFiltered: a Live runner returns the filtered
// event log in oldest-first order; the inner envelope type is "events"
// (plural).
func TestEventsReplyLiveReturnsFiltered(t *testing.T) {
	m := New(nil)
	defer m.Close()

	r := newRunner("alpha", "/tmp/sock", "host:alpha", nil, nil)
	defer r.cancel()
	r.cache.Phase = PhaseLive
	r.cache.EngineTick = 100
	// Cache is newest-first.
	r.cache.Events = []scraper.Envelope{
		makeEvent(7, scraper.EventKill),
		makeEvent(6, scraper.EventDeath),
		makeEvent(5, scraper.EventKill),
	}
	m.runners["alpha"] = r

	bytes, ok := m.EventsReply("alpha", 5, []string{scraper.EventKill})
	if !ok {
		t.Fatal("EventsReply: ok=false")
	}

	var msg websocket.Message
	if err := json.Unmarshal(bytes, &msg); err != nil {
		t.Fatalf("unmarshal websocket.Message: %v", err)
	}
	if msg.Room != "host:alpha" {
		t.Fatalf("msg.room = %q, want %q", msg.Room, "host:alpha")
	}
	var env scraper.Envelope
	if err := json.Unmarshal(msg.Payload, &env); err != nil {
		t.Fatalf("unmarshal scraper.Envelope: %v", err)
	}
	if env.Type != envelopeTypeEvents {
		t.Fatalf("envelope.type = %q, want %q", env.Type, envelopeTypeEvents)
	}
	if env.Instance != "alpha" {
		t.Fatalf("envelope.instance = %q, want %q", env.Instance, "alpha")
	}
	if env.Tick != 100 {
		t.Fatalf("envelope.tick = %d, want 100", env.Tick)
	}

	var payload EventsResponsePayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		t.Fatalf("unmarshal EventsResponsePayload: %v", err)
	}
	if payload.Phase != PhaseLive {
		t.Fatalf("payload.phase = %q, want %q", payload.Phase, PhaseLive)
	}
	if payload.SinceTick != 5 {
		t.Fatalf("payload.since_tick = %d, want 5 (echoed)", payload.SinceTick)
	}
	if len(payload.Events) != 1 {
		t.Fatalf("payload.events len = %d, want 1 (only kill at tick=7)", len(payload.Events))
	}
	if got := eventInnerType(payload.Events[0]); payload.Events[0].Tick != 7 || got != scraper.EventKill {
		t.Fatalf("payload.events[0] tick=%d inner=%q, want kill at tick=7", payload.Events[0].Tick, got)
	}
}

// decodeEventsReply unwraps the websocket.Message + scraper.Envelope wrapper
// and returns the EventsResponsePayload.
func decodeEventsReply(t *testing.T, data []byte) EventsResponsePayload {
	t.Helper()
	var msg websocket.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal websocket.Message: %v", err)
	}
	var env scraper.Envelope
	if err := json.Unmarshal(msg.Payload, &env); err != nil {
		t.Fatalf("unmarshal scraper.Envelope: %v", err)
	}
	if env.Type != envelopeTypeEvents {
		t.Fatalf("envelope.type = %q, want %q", env.Type, envelopeTypeEvents)
	}
	var p EventsResponsePayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		t.Fatalf("unmarshal EventsResponsePayload: %v", err)
	}
	return p
}
