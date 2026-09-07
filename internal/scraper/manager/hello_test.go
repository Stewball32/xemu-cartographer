package manager

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/xemu-cartographer/xc-scraper/scraper"
)

// TestBuildHelloPayloadEmpty: with no runners the Instances list is empty
// (not nil) so a JSON consumer sees `[]` rather than `null`.
func TestBuildHelloPayloadEmpty(t *testing.T) {
	m := New(Options{})
	defer m.Close()

	p := m.BuildHelloPayload()

	if p.ProtocolVersion != scraper.ProtocolVersion {
		t.Fatalf("protocol_version = %d, want %d", p.ProtocolVersion, scraper.ProtocolVersion)
	}
	if p.ServerTime.IsZero() {
		t.Fatal("server_time is zero, want now-ish")
	}
	// Literal strings, not the shared slice or constants — this is the
	// ground-truth pin for the handshake surface, including the three
	// classes both lists historically dropped (game_filtered, event,
	// event_filtered).
	wantClasses := []string{
		"xbox",
		"scenario",
		"game",
		"game_filtered",
		"tick",
		"objects",
		"debug",
		"summary",
		"previous_game",
		"event",
		"event_filtered",
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
	m := New(Options{})
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

// TestHelloEnvelopeBytesRoundtrip: the bare envelope bytes unmarshal back
// through the two manager-side layers (scraper.Envelope → HelloPayload) and
// carry the protocol version + empty instance + tick=0. (The outer
// wire.Message frame is the league adapter's — asserted in
// internal/leaguescraper wireadapter_test.go.)
func TestHelloEnvelopeBytesRoundtrip(t *testing.T) {
	m := New(Options{})
	defer m.Close()

	r := newRunner("alpha", "/tmp/a", "host:alpha", nil, nil, nil)
	defer r.cancel()
	r.cache.StartedAt = time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	m.runners["alpha"] = r

	bytes, ok := m.HelloEnvelopeBytes()
	if !ok {
		t.Fatal("HelloEnvelopeBytes: ok=false")
	}

	var env scraper.Envelope
	if err := json.Unmarshal(bytes, &env); err != nil {
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

// TestHelloEnvelopeMatchesHelloEnvelopeBytes: the package-level HelloEnvelope
// (what the adapter calls with a filtered payload) produces the same envelope
// shape as HelloEnvelopeBytes for the same payload, modulo nothing — both
// stamp instance "" / seq 0 / tick 0.
func TestHelloEnvelopeMatchesHelloEnvelopeBytes(t *testing.T) {
	m := New(Options{})
	defer m.Close()

	payload := m.BuildHelloPayload()
	got, ok := HelloEnvelope(payload)
	if !ok || len(got) == 0 {
		t.Fatalf("HelloEnvelope: ok=%v len=%d", ok, len(got))
	}
	var env scraper.Envelope
	if err := json.Unmarshal(got, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Type != envelopeTypeHello || env.Instance != "" || env.Seq != 0 || env.Tick != 0 {
		t.Fatalf("hello envelope header = %+v, want type=hello instance=\"\" seq=0 tick=0", env)
	}
}
