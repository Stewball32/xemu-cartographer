package manager

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/xemu-cartographer/xc-scraper/scraper"
	"github.com/xemu-cartographer/xc-scraper/wire"
)

// stubEmitter is the test double for the Emitter + Demand ports (ports.go).
// It records every Emit as a roomSend keyed by the room the league emitter
// (internal/leaguescraper) would pick — host:<inst>:<class>, or host:summary
// for instance "" — so assertions keep reading the WS-era room names; Data
// is the bare envelope bytes (no wire.Message frame).
type stubEmitter struct {
	mu        sync.Mutex
	roomSends []roomSend
	// occupied marks per-class rooms that Wants should report as having
	// subscribers (mirrors leaguescraper.wsDemand's room-membership answer).
	// Empty by default; set per-test to make broadcastPoll (PR 15
	// demand-gated) exercise individual classes.
	occupied map[string]bool
}

type roomSend struct {
	Room string
	Data []byte
}

func (s *stubEmitter) Emit(instance, class string, envBytes []byte) {
	room := wire.SummaryRoom
	if instance != "" {
		r, err := wire.RoomForInstanceClass(instance, class)
		if err != nil {
			return
		}
		room = r
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]byte, len(envBytes))
	copy(cp, envBytes)
	s.roomSends = append(s.roomSends, roomSend{Room: room, Data: cp})
}

func (s *stubEmitter) Wants(instance, class string) bool {
	room, err := wire.RoomForInstanceClass(instance, class)
	if err != nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.occupied[room]
}

func (s *stubEmitter) snapshot() []roomSend {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]roomSend, len(s.roomSends))
	copy(out, s.roomSends)
	return out
}

// TestAggregatorBroadcastsOnDirtyTick verifies that an update marks dirty and
// the next coalesce tick produces exactly one broadcast to host:all.
func TestAggregatorBroadcastsOnDirtyTick(t *testing.T) {
	ws := &stubEmitter{}
	a := newAggregator(ws)
	go a.run()
	defer a.stop()

	a.post(summaryUpdate{
		Instance: "smoke1",
		Snapshot: &hostSummary{Instance: "smoke1", Phase: PhaseIdle},
	})

	// Wait long enough for at least one coalesce tick to fire (250ms +
	// margin).
	time.Sleep(aggregatorCoalesce + 100*time.Millisecond)

	sends := ws.snapshot()
	if len(sends) == 0 {
		t.Fatal("aggregator: no broadcast after dirty update")
	}
	if got := sends[0].Room; got != wire.SummaryRoom {
		t.Fatalf("aggregator: broadcast room = %q, want %q", got, wire.SummaryRoom)
	}
}

// TestAggregatorIdleNoBroadcast verifies that no broadcast fires when the
// aggregator has nothing dirty — coalesce tick fires every 250ms but only
// broadcasts when the dirty bit is set.
func TestAggregatorIdleNoBroadcast(t *testing.T) {
	ws := &stubEmitter{}
	a := newAggregator(ws)
	go a.run()
	defer a.stop()

	// Three coalesce ticks with no posts.
	time.Sleep(3*aggregatorCoalesce + 50*time.Millisecond)

	if got := len(ws.snapshot()); got != 0 {
		t.Fatalf("aggregator: idle broadcasts = %d, want 0", got)
	}
}

// TestAggregatorRemovedEvicts verifies that summaryUpdate{Removed:true}
// drops the instance from the cache so a subsequent broadcast doesn't
// include it.
func TestAggregatorRemovedEvicts(t *testing.T) {
	ws := &stubEmitter{}
	a := newAggregator(ws)
	go a.run()
	defer a.stop()

	a.post(summaryUpdate{
		Instance: "alpha",
		Snapshot: &hostSummary{Instance: "alpha", Phase: PhaseLive},
	})
	a.post(summaryUpdate{
		Instance: "bravo",
		Snapshot: &hostSummary{Instance: "bravo", Phase: PhaseReady},
	})
	time.Sleep(aggregatorCoalesce + 100*time.Millisecond)
	startSends := len(ws.snapshot())

	a.post(summaryUpdate{Instance: "alpha", Removed: true})
	time.Sleep(aggregatorCoalesce + 100*time.Millisecond)

	sends := ws.snapshot()
	if len(sends) <= startSends {
		t.Fatal("aggregator: expected a broadcast after Removed update")
	}
	last := sends[len(sends)-1]

	// Verify the most recent broadcast does not include "alpha".
	var env scraper.Envelope
	if err := json.Unmarshal(last.Data, &env); err != nil {
		t.Fatalf("unmarshal scraper.Envelope: %v", err)
	}
	if env.Type != envelopeTypeSummary {
		t.Fatalf("aggregator envelope type = %q, want %q", env.Type, envelopeTypeSummary)
	}
	var sp SummaryPayload
	if err := json.Unmarshal(env.Data, &sp); err != nil {
		t.Fatalf("unmarshal SummaryPayload: %v", err)
	}
	summaries := sp.Hosts
	for _, s := range summaries {
		if s.Instance == "alpha" {
			t.Fatalf("aggregator: alpha still present after Removed: %+v", summaries)
		}
	}
	// bravo should still be present.
	foundBravo := false
	for _, s := range summaries {
		if s.Instance == "bravo" {
			foundBravo = true
		}
	}
	if !foundBravo {
		t.Fatalf("aggregator: bravo evicted along with alpha: %+v", summaries)
	}
}

// TestAggregatorFullSnapshotEachBroadcast verifies that every broadcast
// carries the full hostsCache (OQ2 — full re-broadcast, no diffs).
func TestAggregatorFullSnapshotEachBroadcast(t *testing.T) {
	ws := &stubEmitter{}
	a := newAggregator(ws)
	go a.run()
	defer a.stop()

	a.post(summaryUpdate{
		Instance: "alpha",
		Snapshot: &hostSummary{Instance: "alpha", Phase: PhaseIdle},
	})
	time.Sleep(aggregatorCoalesce + 100*time.Millisecond)

	a.post(summaryUpdate{
		Instance: "bravo",
		Snapshot: &hostSummary{Instance: "bravo", Phase: PhaseReady},
	})
	time.Sleep(aggregatorCoalesce + 100*time.Millisecond)

	sends := ws.snapshot()
	if len(sends) < 2 {
		t.Fatalf("expected at least 2 broadcasts (one per dirty tick), got %d", len(sends))
	}

	// Last broadcast must include both alpha and bravo. Sorted alphabetically
	// per aggregator.snapshot.
	last := sends[len(sends)-1]
	var env scraper.Envelope
	if err := json.Unmarshal(last.Data, &env); err != nil {
		t.Fatalf("unmarshal scraper.Envelope: %v", err)
	}
	if env.Type != envelopeTypeSummary {
		t.Fatalf("aggregator envelope type = %q, want %q", env.Type, envelopeTypeSummary)
	}
	if env.Instance != "" {
		t.Fatalf("aggregator envelope instance = %q, want empty (summary is multi-instance)", env.Instance)
	}
	var sp SummaryPayload
	if err := json.Unmarshal(env.Data, &sp); err != nil {
		t.Fatalf("unmarshal SummaryPayload: %v", err)
	}
	summaries := sp.Hosts
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d: %+v", len(summaries), summaries)
	}
	if summaries[0].Instance != "alpha" || summaries[1].Instance != "bravo" {
		t.Fatalf("expected sorted [alpha, bravo], got [%s, %s]", summaries[0].Instance, summaries[1].Instance)
	}
}

// TestAggregatorJoinReplay returns a single envelope-message representing
// the current cache. Used by join_room handler when a client joins host:all.
func TestAggregatorJoinReplay(t *testing.T) {
	ws := &stubEmitter{}
	a := newAggregator(ws)
	go a.run()
	defer a.stop()

	a.post(summaryUpdate{
		Instance: "alpha",
		Snapshot: &hostSummary{Instance: "alpha", Phase: PhaseLive, Map: "bloodgulch"},
	})
	// Wait for apply but not necessarily for a broadcast.
	time.Sleep(50 * time.Millisecond)

	out := a.joinReplay()
	if len(out) != 1 {
		t.Fatalf("joinReplay() returned %d messages, want 1", len(out))
	}

	var msg wire.Message
	if err := json.Unmarshal(out[0], &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Room != wire.SummaryRoom {
		t.Fatalf("joinReplay room = %q, want %q", msg.Room, wire.SummaryRoom)
	}
	var env scraper.Envelope
	if err := json.Unmarshal(msg.Payload, &env); err != nil {
		t.Fatalf("unmarshal scraper.Envelope: %v", err)
	}
	if env.Type != envelopeTypeSummary {
		t.Fatalf("joinReplay envelope type = %q, want %q", env.Type, envelopeTypeSummary)
	}
}

// TestRenderScoreSummary covers the team-score formatting helper. FFA returns
// empty so the host-list UI can fall back to phase / map.
func TestRenderScoreSummary(t *testing.T) {
	cases := []struct {
		name string
		gd   *scraper.GameData
		want string
	}{
		{"nil", nil, ""},
		{"no-teams", &scraper.GameData{}, ""},
		{"two-teams", &scraper.GameData{TeamScores: []scraper.TeamScore{
			{Team: 0, Score: 12}, {Team: 1, Score: 9},
		}}, "12 — 9"},
		{"single-team", &scraper.GameData{TeamScores: []scraper.TeamScore{
			{Team: 0, Score: 5},
		}}, "5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := renderScoreSummary(tc.gd); got != tc.want {
				t.Fatalf("renderScoreSummary: got %q, want %q", got, tc.want)
			}
		})
	}
}
