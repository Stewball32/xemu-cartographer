package leaguescraper

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/xemu-cartographer/xc-scraper/wire"
	"github.com/xemu-cartographer/xemu-cartographer/internal/guards"
)

// fakeHub is a minimal wsiface.Service double: it records SendToRoomRaw
// calls and answers RoomHasMembers from the occupied set.
type fakeHub struct {
	mu       sync.Mutex
	sends    []hubSend
	occupied map[string]bool
}

type hubSend struct {
	Room string
	Data []byte
}

func (h *fakeHub) BroadcastRaw([]byte)          {}
func (h *fakeHub) SendToUserRaw(string, []byte) {}
func (h *fakeHub) IsConnected(string) bool      { return false }
func (h *fakeHub) IsInRoom(string, string) bool { return false }
func (h *fakeHub) UserRooms(string) []string    { return nil }
func (h *fakeHub) RoomHasMembers(room string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.occupied[room]
}
func (h *fakeHub) SendToRoomRaw(room string, data []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	cp := make([]byte, len(data))
	copy(cp, data)
	h.sends = append(h.sends, hubSend{Room: room, Data: cp})
}
func (h *fakeHub) snapshot() []hubSend {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]hubSend, len(h.sends))
	copy(out, h.sends)
	return out
}

// TestEmitterRoutesPerClassAndFrames: a per-instance class lands on
// host:<inst>:<class> framed as wire.Message{Type:"scraper", Room, Payload}
// with the envelope bytes carried verbatim; the summary class (instance "")
// lands on host:summary.
func TestEmitterRoutesPerClassAndFrames(t *testing.T) {
	hub := &fakeHub{}
	svc := &guards.Services{WS: hub}
	em := NewEmitter(svc)

	env := []byte(`{"v":2,"type":"tick","instance":"alpha","seq":7}`)
	em.Emit("alpha", "tick", env)
	em.Emit("", wire.ClassSummary, []byte(`{"v":2,"type":"summary"}`))

	sends := hub.snapshot()
	if len(sends) != 2 {
		t.Fatalf("want 2 sends, got %d", len(sends))
	}
	if sends[0].Room != "host:alpha:tick" {
		t.Fatalf("tick room = %q, want host:alpha:tick", sends[0].Room)
	}
	if sends[1].Room != wire.SummaryRoom {
		t.Fatalf("summary room = %q, want %q", sends[1].Room, wire.SummaryRoom)
	}
	var msg wire.Message
	if err := json.Unmarshal(sends[0].Data, &msg); err != nil {
		t.Fatalf("unmarshal wire.Message: %v", err)
	}
	if msg.Type != wire.TypeScraper || msg.Room != "host:alpha:tick" {
		t.Fatalf("frame = %+v", msg)
	}
	if string(msg.Payload) != string(env) {
		t.Fatalf("payload = %s, want %s", msg.Payload, env)
	}
}

// TestEmitterDropsWithoutHubOrRoom: no hub (svc nil / svc.WS nil) and an
// unroutable (instance, class) pair are both silent drops.
func TestEmitterDropsWithoutHubOrRoom(t *testing.T) {
	NewEmitter(nil).Emit("alpha", "tick", []byte(`{}`))
	NewEmitter(&guards.Services{}).Emit("alpha", "tick", []byte(`{}`))

	hub := &fakeHub{}
	em := NewEmitter(&guards.Services{WS: hub})
	em.Emit("alpha", "not-a-class", []byte(`{}`))
	em.Emit("all", "tick", []byte(`{}`)) // reserved instance name
	if got := len(hub.snapshot()); got != 0 {
		t.Fatalf("unroutable pairs reached the hub: %d sends", got)
	}
}

// TestDemandFollowsRoomMembership: Wants mirrors the per-class room's
// membership; an unroutable pair is never wanted; no hub yet is permissive.
func TestDemandFollowsRoomMembership(t *testing.T) {
	hub := &fakeHub{occupied: map[string]bool{"host:alpha:game": true}}
	d := NewDemand(&guards.Services{WS: hub})

	if !d.Wants("alpha", "game") {
		t.Fatal("occupied room: want true")
	}
	if d.Wants("alpha", "tick") {
		t.Fatal("empty room: want false")
	}
	if d.Wants("alpha", "not-a-class") {
		t.Fatal("unroutable class: want false")
	}
	if d.Wants("all", "game") {
		t.Fatal("reserved instance: want false")
	}
	if !NewDemand(nil).Wants("alpha", "tick") {
		t.Fatal("nil services: want permissive true")
	}
	if !NewDemand(&guards.Services{}).Wants("alpha", "tick") {
		t.Fatal("nil hub: want permissive true")
	}
}
