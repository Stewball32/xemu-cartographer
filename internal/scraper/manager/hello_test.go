package manager

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/websocket"
)

// TestBuildHelloPayloadEmpty: with no runners the Instances list is empty
// (not nil) so a JSON consumer sees `[]` rather than `null`.
func TestBuildHelloPayloadEmpty(t *testing.T) {
	m := New(nil)
	defer m.Close()

	p := m.BuildHelloPayload()

	if p.ProtocolVersion != scraper.ProtocolVersion {
		t.Fatalf("protocol_version = %d, want %d", p.ProtocolVersion, scraper.ProtocolVersion)
	}
	if p.ServerTime.IsZero() {
		t.Fatal("server_time is zero, want now-ish")
	}
	wantClasses := []string{
		envelopeTypeXbox,
		envelopeTypeScenario,
		envelopeTypeGame,
		envelopeTypeTick,
		envelopeTypeObjects,
		envelopeTypeDebug,
		envelopeTypeSummary,
		envelopeTypePreviousGame,
	}
	if len(p.Classes) != len(wantClasses) {
		t.Fatalf("classes = %v (len %d), want %v (len %d)", p.Classes, len(p.Classes), wantClasses, len(wantClasses))
	}
	for i, c := range wantClasses {
		if p.Classes[i] != c {
			t.Fatalf("classes[%d] = %q, want %q", i, p.Classes[i], c)
		}
	}
	if p.Instances == nil {
		t.Fatal("instances = nil, want []HelloInstance{} so JSON marshals as []")
	}
	if len(p.Instances) != 0 {
		t.Fatalf("instances len = %d, want 0 with no runners", len(p.Instances))
	}
}

// TestBuildHelloPayloadWithRunners: each runner contributes one entry to
// Instances, sorted by name (inherited from m.List), each carrying the
// runner's StartedAt — the value clients use for restart detection.
func TestBuildHelloPayloadWithRunners(t *testing.T) {
	m := New(nil)
	defer m.Close()

	started := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)

	// Names chosen so insertion order ≠ sort order — proves the sort.
	rZ := newRunner("zulu", "/tmp/z", "host:zulu", nil, nil, nil)
	defer rZ.cancel()
	rZ.cache.StartedAt = started.Add(2 * time.Hour)

	rA := newRunner("alpha", "/tmp/a", "host:alpha", nil, nil, nil)
	defer rA.cancel()
	rA.cache.StartedAt = started

	m.runners["zulu"] = rZ
	m.runners["alpha"] = rA

	p := m.BuildHelloPayload()

	if len(p.Instances) != 2 {
		t.Fatalf("instances len = %d, want 2", len(p.Instances))
	}
	if p.Instances[0].Name != "alpha" || p.Instances[1].Name != "zulu" {
		t.Fatalf("instance order = [%q, %q], want [alpha, zulu]", p.Instances[0].Name, p.Instances[1].Name)
	}
	if !p.Instances[0].StartedAt.Equal(started) {
		t.Fatalf("alpha started_at = %v, want %v", p.Instances[0].StartedAt, started)
	}
	if !p.Instances[1].StartedAt.Equal(started.Add(2 * time.Hour)) {
		t.Fatalf("zulu started_at = %v, want %v", p.Instances[1].StartedAt, started.Add(2*time.Hour))
	}
}

// TestHelloEnvelopeBytesRoundtrip: the wire bytes unmarshal back through the
// expected three layers (websocket.Message → scraper.Envelope → HelloPayload)
// and carry the protocol version + empty instance + tick=0.
func TestHelloEnvelopeBytesRoundtrip(t *testing.T) {
	m := New(nil)
	defer m.Close()

	r := newRunner("alpha", "/tmp/a", "host:alpha", nil, nil, nil)
	defer r.cancel()
	r.cache.StartedAt = time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	m.runners["alpha"] = r

	bytes, ok := m.HelloEnvelopeBytes()
	if !ok {
		t.Fatal("HelloEnvelopeBytes: ok=false")
	}

	var msg websocket.Message
	if err := json.Unmarshal(bytes, &msg); err != nil {
		t.Fatalf("unmarshal websocket.Message: %v", err)
	}
	if msg.Type != "scraper" {
		t.Fatalf("msg.type = %q, want %q", msg.Type, "scraper")
	}
	if msg.Room != "" {
		t.Fatalf("msg.room = %q, want empty (hello is not per-room)", msg.Room)
	}

	var env scraper.Envelope
	if err := json.Unmarshal(msg.Payload, &env); err != nil {
		t.Fatalf("unmarshal scraper.Envelope: %v", err)
	}
	if env.V != scraper.ProtocolVersion {
		t.Fatalf("env.v = %d, want %d", env.V, scraper.ProtocolVersion)
	}
	if env.Type != envelopeTypeHello {
		t.Fatalf("env.type = %q, want %q", env.Type, envelopeTypeHello)
	}
	if env.Instance != "" {
		t.Fatalf("env.instance = %q, want empty", env.Instance)
	}
	if env.Tick != 0 {
		t.Fatalf("env.tick = %d, want 0", env.Tick)
	}

	var payload HelloPayload
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		t.Fatalf("unmarshal HelloPayload: %v", err)
	}
	if payload.ProtocolVersion != scraper.ProtocolVersion {
		t.Fatalf("payload.protocol_version = %d, want %d", payload.ProtocolVersion, scraper.ProtocolVersion)
	}
	if len(payload.Instances) != 1 || payload.Instances[0].Name != "alpha" {
		t.Fatalf("payload.instances = %+v, want one entry [alpha]", payload.Instances)
	}
}

// TestSendHelloOn: SendHelloOn invokes the send function exactly once with
// the same bytes HelloEnvelopeBytes would have returned (modulo the captured
// server_time, which advances per call).
func TestSendHelloOn(t *testing.T) {
	m := New(nil)
	defer m.Close()

	calls := 0
	var got []byte
	m.SendHelloOn(func(data []byte) {
		calls++
		got = data
	})

	if calls != 1 {
		t.Fatalf("SendHelloOn: send called %d times, want 1", calls)
	}
	if len(got) == 0 {
		t.Fatal("SendHelloOn: send received empty bytes")
	}
	// Sanity: the bytes parse as a websocket.Message wrapping a hello envelope.
	var msg websocket.Message
	if err := json.Unmarshal(got, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Type != "scraper" {
		t.Fatalf("msg.type = %q, want %q", msg.Type, "scraper")
	}
}
