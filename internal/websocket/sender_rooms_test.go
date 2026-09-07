package websocket

import (
	"sort"
	"testing"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/guards"
	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
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
func addTestClient(h *Hub, user *core.Record, roomNames ...string) *Client {
	c := &Client{hub: h, send: make(chan []byte, sendBufSize), user: user}
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

func userRecord(id string) *core.Record {
	rec := core.NewRecord(core.NewBaseCollection("users"))
	rec.Id = id
	return rec
}

// TestRequestState_RepliesOnlySenderRooms is the regression test for the
// anonymous-identity room bug: request_state used to key the caller's
// membership on UserRooms(UserID), and since every anonymous client shares
// UserID "" (and one user can have several tabs), one connection's resync
// replayed the UNION of all same-identity connections' rooms. The Event's
// per-sender Rooms capability must replay exactly the sender's own rooms.
func TestRequestState_RepliesOnlySenderRooms(t *testing.T) {
	anon := func() *core.Record { return nil }
	sameUser := userRecord("user1")

	tests := []struct {
		name        string
		senderUser  *core.Record
		senderRooms []string
		otherUser   *core.Record
		otherRooms  []string
		wantSender  []string
	}{
		{
			name:        "two anonymous clients in different rooms",
			senderUser:  anon(),
			senderRooms: []string{"host:pod-a"},
			otherUser:   anon(),
			otherRooms:  []string{"host:pod-b"},
			wantSender:  []string{"replay instance pod-a"},
		},
		{
			name:        "two tabs of the same user in different rooms",
			senderUser:  sameUser,
			senderRooms: []string{"host:pod-a"},
			otherUser:   sameUser,
			otherRooms:  []string{"host:pod-b"},
			wantSender:  []string{"replay instance pod-a"},
		},
		{
			name:        "sender's own multi-room set replays fully",
			senderUser:  anon(),
			senderRooms: []string{"host:pod-a", "host:pod-b:game", "host:summary"},
			otherUser:   anon(),
			otherRooms:  []string{"host:pod-c"},
			wantSender:  []string{"replay class pod-b game", "replay hostall", "replay instance pod-a"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHub(nil)
			// WS is wired like production so a regression to the
			// UserRooms(UserID) lookup fails by cross-replay, not by a
			// nil-service early return.
			h.SetServices(&guards.Services{Scraper: stubReplayScraper{}, WS: h})
			sender := addTestClient(h, tc.senderUser, tc.senderRooms...)
			other := addTestClient(h, tc.otherUser, tc.otherRooms...)

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
