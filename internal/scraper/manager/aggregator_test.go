package manager

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/guards"
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/websocket"
	"github.com/Stewball32/xemu-cartographer/internal/websocket/rooms"
)

// stubWS captures SendToRoomRaw calls for assertions. Implements wsiface.Service
// surface area the aggregator and runner touch (only SendToRoomRaw is used for
// scraper broadcasts; the other methods exist to satisfy the interface).
type stubWS struct {
	mu        sync.Mutex
	roomSends []roomSend
}

type roomSend struct {
	Room string
	Data []byte
}

func (s *stubWS) BroadcastRaw([]byte)             {}
func (s *stubWS) SendToUserRaw(string, []byte)    {}
func (s *stubWS) IsConnected(string) bool          { return false }
func (s *stubWS) IsInRoom(string, string) bool     { return false }
func (s *stubWS) UserRooms(string) []string        { return nil }
func (s *stubWS) SendToRoomRaw(room string, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]byte, len(data))
	copy(cp, data)
	s.roomSends = append(s.roomSends, roomSend{Room: room, Data: cp})
}

func (s *stubWS) snapshot() []roomSend {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]roomSend, len(s.roomSends))
	copy(out, s.roomSends)
	return out
}

// TestAggregatorBroadcastsOnDirtyTick verifies that an update marks dirty and
// the next coalesce tick produces exactly one broadcast to host:all.
func TestAggregatorBroadcastsOnDirtyTick(t *testing.T) {
	ws := &stubWS{}
	a := newAggregator(&guards.Services{WS: ws})
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
	if got := sends[0].Room; got != rooms.HostAllRoom {
		t.Fatalf("aggregator: broadcast room = %q, want %q", got, rooms.HostAllRoom)
	}
}

// TestAggregatorIdleNoBroadcast verifies that no broadcast fires when the
// aggregator has nothing dirty — coalesce tick fires every 250ms but only
// broadcasts when the dirty bit is set.
func TestAggregatorIdleNoBroadcast(t *testing.T) {
	ws := &stubWS{}
	a := newAggregator(&guards.Services{WS: ws})
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
	ws := &stubWS{}
	a := newAggregator(&guards.Services{WS: ws})
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
	var msg websocket.Message
	if err := json.Unmarshal(last.Data, &msg); err != nil {
		t.Fatalf("unmarshal websocket.Message: %v", err)
	}
	var env scraper.Envelope
	if err := json.Unmarshal(msg.Payload, &env); err != nil {
		t.Fatalf("unmarshal scraper.Envelope: %v", err)
	}
	var summaries []hostSummary
	if err := json.Unmarshal(env.Payload, &summaries); err != nil {
		t.Fatalf("unmarshal []hostSummary: %v", err)
	}
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
	ws := &stubWS{}
	a := newAggregator(&guards.Services{WS: ws})
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
	var msg websocket.Message
	if err := json.Unmarshal(last.Data, &msg); err != nil {
		t.Fatalf("unmarshal websocket.Message: %v", err)
	}
	var env scraper.Envelope
	if err := json.Unmarshal(msg.Payload, &env); err != nil {
		t.Fatalf("unmarshal scraper.Envelope: %v", err)
	}
	if env.Instance != "all" {
		t.Fatalf("aggregator envelope instance = %q, want %q", env.Instance, "all")
	}
	var summaries []hostSummary
	if err := json.Unmarshal(env.Payload, &summaries); err != nil {
		t.Fatalf("unmarshal []hostSummary: %v", err)
	}
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
	ws := &stubWS{}
	a := newAggregator(&guards.Services{WS: ws})
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

	var msg websocket.Message
	if err := json.Unmarshal(out[0], &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Room != rooms.HostAllRoom {
		t.Fatalf("joinReplay room = %q, want %q", msg.Room, rooms.HostAllRoom)
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
