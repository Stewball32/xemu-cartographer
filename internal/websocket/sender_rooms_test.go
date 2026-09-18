package websocket

import (
	"sort"
	"testing"

	"github.com/xemu-cartographer/xemu-cartographer/internal/authz"
	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/authztest"
	"github.com/xemu-cartographer/xemu-cartographer/internal/guards"
	scraperiface "github.com/xemu-cartographer/xemu-cartographer/internal/guards/interfaces/scraper"
)

// stubReplayScraper satisfies scraperiface.Service via the embedded (nil)
// interface — only the join-replay methods request_state dispatches to are
// implemented, returning bytes tagged with the room they answer for so a test
// can tell exactly which rooms were replayed.
type stubReplayScraper struct {
	scraperiface.Service
}

func (stubReplayScraper) JoinReplayForInstance(name string) [][]byte {
	return [][]byte{[]byte("replay instance " + name)}
}

func (stubReplayScraper) JoinReplayForInstanceClass(name, class string) [][]byte {
	return [][]byte{[]byte("replay class " + name + " " + class)}
}

func (stubReplayScraper) JoinReplayForHostAll() [][]byte {
	return [][]byte{[]byte("replay hostall")}
}

// addTestClient registers a fake connection directly in the hub's indexes —
// bypassing Run() so the test stays synchronous — and joins it to rooms.
func addTestClient(h *Hub, p authz.Principal, roomNames ...string) *Client {
	c := newClient(h, nil, p)
	h.mu.Lock()
	h.clients[c] = true
	if uid := c.UserID(); uid != "" {
		if h.users[uid] == nil {
			h.users[uid] = make(map[*Client]bool)
		}
		h.users[uid][c] = true
	}
	for _, room := range roomNames {
		if h.rooms[room] == nil {
			h.rooms[room] = make(map[*Client]bool)
		}
		h.rooms[room][c] = true
	}
	h.mu.Unlock()
	return c
}

// drainSend collects everything queued on a client's send channel, sorted
// (room iteration order is map-random).
func drainSend(c *Client) []string {
	var got []string
	for {
		select {
		case data := <-c.send:
			got = append(got, string(data))
		default:
			sort.Strings(got)
			return got
		}
	}
}

// stateScopes lets a principal pass request_state's per-room checks by
// scope alone (scraper.state on every instance, room.join on the aggregate
// feeds) so the test below is about membership, not about authz.
var stateScopes = []string{"scraper.*", "room.join:*"}

// machineKey models one connection holding a machine key: no users record
// behind it, so every such client shares UserID "" — the same identity
// collision the old anonymous door had.
func machineKey(kid string) authz.Principal {
	return authz.Principal{Kind: authz.KindMachine, ID: kid, Scopes: authz.CanonScopes(stateScopes)}
}

// userPrincipal models a logged-in user (one per tab) carrying stateScopes.
func userPrincipal(id string) authz.Principal {
	return authz.Principal{Kind: authz.KindPBUser, ID: id, UserID: id, Collection: "users", Scopes: authz.CanonScopes(stateScopes)}
}

// TestRequestState_RepliesOnlySenderRooms is the regression test for the
// shared-identity room bug: request_state used to key the caller's
// membership on UserRooms(UserID), and since every connection without a
// users record shares UserID "" (and one user can have several tabs), one
// connection's resync replayed the UNION of all same-identity connections'
// rooms. The Event's per-sender Rooms capability must replay exactly the
// sender's own rooms.
func TestRequestState_RepliesOnlySenderRooms(t *testing.T) {
	sameUser := userPrincipal("user1")

	tests := []struct {
		name        string
		sender      authz.Principal
		senderRooms []string
		other       authz.Principal
		otherRooms  []string
		wantSender  []string
	}{
		{
			name:        "two userless machine keys in different rooms",
			sender:      machineKey("k1"),
			senderRooms: []string{"host:pod-a"},
			other:       machineKey("k2"),
			otherRooms:  []string{"host:pod-b"},
			wantSender:  []string{"replay instance pod-a"},
		},
		{
			name:        "two tabs of the same user in different rooms",
			sender:      sameUser,
			senderRooms: []string{"host:pod-a"},
			other:       sameUser,
			otherRooms:  []string{"host:pod-b"},
			wantSender:  []string{"replay instance pod-a"},
		},
		{
			name:        "sender's own multi-room set replays fully",
			sender:      machineKey("k1"),
			senderRooms: []string{"host:pod-a", "host:pod-b:game", "host:summary"},
			other:       machineKey("k2"),
			otherRooms:  []string{"host:pod-c"},
			wantSender:  []string{"replay class pod-b game", "replay hostall", "replay instance pod-a"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHub(nil)
			// WS is wired like production so a regression to the
			// UserRooms(UserID) lookup fails by cross-replay, not by a
			// nil-service early return. Deps are a zero FakeDeps: every
			// per-room decision passes on the principal's scopes alone.
			h.SetServices(&guards.Services{Scraper: stubReplayScraper{}, WS: h})
			h.deps = &authztest.FakeDeps{}
			sender := addTestClient(h, tc.sender, tc.senderRooms...)
			other := addTestClient(h, tc.other, tc.otherRooms...)

			h.dispatch(incomingMsg{msg: Message{Type: "request_state"}, sender: sender})

			if got := drainSend(sender); !equalStrings(got, tc.wantSender) {
				t.Errorf("sender replies = %q, want %q", got, tc.wantSender)
			}
			if got := drainSend(other); len(got) != 0 {
				t.Errorf("non-sender received %q, want nothing", got)
			}
		})
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
