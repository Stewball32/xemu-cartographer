package handlers

import (
	"sort"
	"testing"

	"github.com/xemu-cartographer/xemu-cartographer/internal/authz"
	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/authztest"
	"github.com/xemu-cartographer/xemu-cartographer/internal/guards"
)

// TestHandleRequestState_PerRoomCan: membership selects the rooms, then
// each replay is re-decided — scraper.state on the instance for host rooms,
// room.join on the aggregate room itself for host:all / host:summary — so a
// sender whose access lapsed since it joined gets nothing for that room
// while the re-resolve tick catches up. The aggregate feeds are decided on
// the room the sender actually holds: a key scoped to only one of the two
// spellings is replayed for that one (k5, k6), never for the other.
func TestHandleRequestState_PerRoomCan(t *testing.T) {
	senderRooms := []string{"host:pod-a", "host:pod-b:tick", "host:summary", "host:all", "admin:dashboard", "public:lobby"}
	deps := &authztest.FakeDeps{Rostered: map[string]bool{authztest.Key("u1", "pod-a"): true}}
	tests := []struct {
		name     string
		p        authz.Principal
		deps     authz.Deps
		wantSent []string
	}{
		{"machine scraper only", machine("k1", "scraper.*"), deps, []string{"class:pod-b:tick", "instance:pod-a"}},
		{"machine scraper and rooms", machine("k2", "scraper.*", "room.join:*"), deps, []string{"class:pod-b:tick", "hostall", "hostall", "instance:pod-a"}},
		{"machine rooms only", machine("k3", "room.join:*"), deps, []string{"hostall", "hostall"}},
		{"machine no scopes", machine("k4"), deps, nil},
		{"machine host:all only", machine("k5", "room.join:host:all"), deps, []string{"hostall"}},
		{"machine host:summary only", machine("k6", "room.join:host:summary"), deps, []string{"hostall"}},
		{"user rostered in pod-a", pbUser("u1"), deps, []string{"instance:pod-a"}},
		{"user rostered nowhere", pbUser("u2"), deps, nil},
		{"admin user", pbUser("a1", adminScopes...), deps, []string{"class:pod-b:tick", "hostall", "hostall", "instance:pod-a"}},
		{"spectator bound to pod-b", boundKey(authz.KindSpectator, "s1", "pod-b", "scraper.state:*", "room.join:*"), deps, []string{"class:pod-b:tick"}},
		{"device bound elsewhere", boundKey(authz.KindDevice, "d1", "pod-z", "scraper.*"), deps, nil},
		{"nobody", authz.Nobody(), deps, nil},
		{"superuser", authz.Superuser("su"), deps, []string{"class:pod-b:tick", "hostall", "hostall", "instance:pod-a"}},
		{"superuser with nil deps", authz.Superuser("su"), nil, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubReplayScraper{}
			var sent []string
			handleRequestState(&Event{
				Services:  &guards.Services{Scraper: stub},
				Authz:     tc.deps,
				Principal: tc.p,
				Rooms:     func() []string { return senderRooms },
				SendRaw:   func(b []byte) { sent = append(sent, string(b)) },
			})
			sort.Strings(sent)
			if !equalStrings(sent, tc.wantSent) {
				t.Errorf("replayed %q, want %q", sent, tc.wantSent)
			}
		})
	}

	// No Rooms capability: nothing is replayed even for a superuser.
	stub := &stubReplayScraper{}
	var sent []string
	handleRequestState(&Event{
		Services:  &guards.Services{Scraper: stub},
		Authz:     deps,
		Principal: authz.Superuser("su"),
		SendRaw:   func(b []byte) { sent = append(sent, string(b)) },
	})
	if len(sent) != 0 {
		t.Errorf("nil Rooms replayed %q, want nothing", sent)
	}
}

// TestHandleRequestState_AggregateDecidedOnRoomHeld pins that an aggregate
// replay is decided against the room the sender is in, not against
// host:summary on its behalf: a key scoped room.join:host:all is replayed
// for host:all (the room join_room admitted it to) and gets nothing for
// host:summary, and the mirror holds for a host:summary-only key. Deciding
// every aggregate on host:summary both starved host:all-only keys and let a
// host:summary scope answer for a room it never covered.
func TestHandleRequestState_AggregateDecidedOnRoomHeld(t *testing.T) {
	deps := &authztest.FakeDeps{}
	tests := []struct {
		name  string
		p     authz.Principal
		rooms []string
		want  int
	}{
		{"host:all key in host:all", machine("k1", "room.join:host:all"), []string{"host:all"}, 1},
		{"host:all key in host:summary", machine("k1", "room.join:host:all"), []string{"host:summary"}, 0},
		{"host:summary key in host:summary", machine("k2", "room.join:host:summary"), []string{"host:summary"}, 1},
		{"host:summary key in host:all", machine("k2", "room.join:host:summary"), []string{"host:all"}, 0},
		{"host:all key in both", machine("k1", "room.join:host:all"), []string{"host:all", "host:summary"}, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var sent []string
			handleRequestState(&Event{
				Services:  &guards.Services{Scraper: &stubReplayScraper{}},
				Authz:     deps,
				Principal: tc.p,
				Rooms:     func() []string { return tc.rooms },
				SendRaw:   func(b []byte) { sent = append(sent, string(b)) },
			})
			if len(sent) != tc.want {
				t.Errorf("replayed %q, want %d aggregate replay(s)", sent, tc.want)
			}
		})
	}
}
