package manager

import (
	"testing"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
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
		envelopeTypeCurrentState,
		envelopeTypeStateUpdate,
		envelopeTypeEvent,
		envelopeTypeEvents,
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
	rZ := newRunner("zulu", "/tmp/z", "host:zulu", nil, nil)
	defer rZ.cancel()
	rZ.cache.StartedAt = started.Add(2 * time.Hour)

	rA := newRunner("alpha", "/tmp/a", "host:alpha", nil, nil)
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
