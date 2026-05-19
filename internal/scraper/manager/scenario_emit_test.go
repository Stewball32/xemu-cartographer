package manager

import (
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/guards"
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// TestMaybeEmitScenarioReEmitsOnFingerprintChange exercises the edge-
// trigger path: an empty cache stays silent; promoting GameData with
// scenario fields fires exactly one envelope; repeat calls without
// further mutation stay silent.
func TestMaybeEmitScenarioReEmitsOnFingerprintChange(t *testing.T) {
	ws := &stubWS{}
	svc := &guards.Services{WS: ws}
	r := newTestRunner("alpha")
	defer r.cancel()

	// 1. Bare cache — fingerprint is 0, no emission.
	r.maybeEmitScenario(svc)
	if got := len(ws.snapshot()); got != 0 {
		t.Fatalf("empty cache: want 0 sends, got %d", got)
	}

	// 2. Populate scenario-derived fields. First call should emit one
	// scenario envelope to host:alpha:scenario.
	r.withCache(func(c *instanceCache) {
		c.GameData = &scraper.GameData{
			Map: "bloodgulch",
			PlayerSpawns: []scraper.StaticPlayerSpawn{
				{Index: 0}, {Index: 1}, {Index: 2},
			},
			Fog: &scraper.StaticFog{},
		}
	})
	r.maybeEmitScenario(svc)
	sends := ws.snapshot()
	if len(sends) != 1 {
		t.Fatalf("after fill: want 1 send, got %d (%+v)", len(sends), sends)
	}
	if sends[0].Room != "host:alpha:scenario" {
		t.Fatalf("after fill: want host:alpha:scenario, got %q", sends[0].Room)
	}

	// 3. Same cache, no fingerprint change — no second emission.
	r.maybeEmitScenario(svc)
	if got := len(ws.snapshot()); got != 1 {
		t.Fatalf("after no-op call: want still 1 send, got %d", got)
	}

	// 4. Add another spawn — fingerprint shifts, one more envelope fires.
	r.withCache(func(c *instanceCache) {
		c.GameData.PlayerSpawns = append(c.GameData.PlayerSpawns, scraper.StaticPlayerSpawn{Index: 3})
	})
	r.maybeEmitScenario(svc)
	if got := len(ws.snapshot()); got != 2 {
		t.Fatalf("after second mutation: want 2 sends, got %d", got)
	}
}

// TestBroadcastSnapshotUpdatesScenarioFingerprint verifies that
// broadcastSnapshot (called on phase transitions) syncs the fingerprint
// so a downstream maybeEmitScenario is a no-op — protects against double
// emission on Ready→Live.
func TestBroadcastSnapshotUpdatesScenarioFingerprint(t *testing.T) {
	ws := &stubWS{}
	svc := &guards.Services{WS: ws}
	r := newTestRunner("bravo")
	defer r.cancel()
	r.withCache(func(c *instanceCache) {
		c.Phase = PhaseLive
		c.GameData = &scraper.GameData{
			Map:          "bloodgulch",
			PlayerSpawns: []scraper.StaticPlayerSpawn{{Index: 0}, {Index: 1}},
		}
		c.LatestTick = &scraper.TickPayload{}
	})

	if r.lastScenarioFingerprint != 0 {
		t.Fatalf("pre-snapshot: want fingerprint 0, got %d", r.lastScenarioFingerprint)
	}

	r.broadcastSnapshot(svc)

	if r.lastScenarioFingerprint == 0 {
		t.Fatalf("post-snapshot: fingerprint should be non-zero")
	}
	want := computeScenarioFingerprint(&scraper.GameData{
		PlayerSpawns: []scraper.StaticPlayerSpawn{{Index: 0}, {Index: 1}},
	})
	if r.lastScenarioFingerprint != want {
		t.Fatalf("post-snapshot: want fingerprint %d, got %d", want, r.lastScenarioFingerprint)
	}

	// broadcastSnapshot already emitted the scenario envelope; count
	// before the no-op call so we can verify maybeEmitScenario doesn't
	// double-emit.
	beforeCount := len(ws.snapshot())
	r.maybeEmitScenario(svc)
	if got := len(ws.snapshot()); got != beforeCount {
		t.Fatalf("maybeEmitScenario after snapshot: want %d sends (no-op), got %d", beforeCount, got)
	}
}

// TestReleaseReaderResetsScenarioFingerprint: releaseReader is the
// Ready→Idle / Live→Idle exit path. Clearing the fingerprint there
// guarantees the next Idle→Ready→fill cycle re-emits scenario for the
// new match.
func TestReleaseReaderResetsScenarioFingerprint(t *testing.T) {
	r := newTestRunner("charlie")
	defer r.cancel()
	r.withCache(func(c *instanceCache) {
		c.GameData = &scraper.GameData{
			PlayerSpawns: []scraper.StaticPlayerSpawn{{Index: 0}},
		}
	})
	r.lastScenarioFingerprint = computeScenarioFingerprint(r.cache.GameData)
	if r.lastScenarioFingerprint == 0 {
		t.Fatal("setup: fingerprint should be non-zero before releaseReader")
	}

	r.releaseReader()

	if r.lastScenarioFingerprint != 0 {
		t.Fatalf("after releaseReader: want fingerprint 0, got %d", r.lastScenarioFingerprint)
	}
}

// TestComputeScenarioFingerprintDistinguishesCacheStates is a unit
// check on the packing — three meaningfully-different GameDatas must
// produce three different fingerprints, and identical inputs must
// produce identical fingerprints.
func TestComputeScenarioFingerprintDistinguishesCacheStates(t *testing.T) {
	empty := computeScenarioFingerprint(nil)
	if empty != 0 {
		t.Fatalf("nil: want 0, got %d", empty)
	}

	emptyGD := computeScenarioFingerprint(&scraper.GameData{})
	if emptyGD != 0 {
		t.Fatalf("empty GameData: want 0, got %d", emptyGD)
	}

	withSpawns := computeScenarioFingerprint(&scraper.GameData{
		PlayerSpawns: []scraper.StaticPlayerSpawn{{Index: 0}, {Index: 1}},
	})
	if withSpawns == 0 {
		t.Fatal("with spawns: want non-zero fingerprint")
	}

	withFog := computeScenarioFingerprint(&scraper.GameData{
		PlayerSpawns: []scraper.StaticPlayerSpawn{{Index: 0}, {Index: 1}},
		Fog:          &scraper.StaticFog{},
	})
	if withFog == withSpawns {
		t.Fatal("adding fog should change fingerprint")
	}

	again := computeScenarioFingerprint(&scraper.GameData{
		PlayerSpawns: []scraper.StaticPlayerSpawn{{Index: 9}, {Index: 99}},
	})
	if again != withSpawns {
		t.Fatalf("same lengths: fingerprint should be identical (%d != %d)", again, withSpawns)
	}
}
