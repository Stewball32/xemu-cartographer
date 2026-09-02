package handlers

import (
	"strings"

	"github.com/Stewball32/xemu-cartographer/internal/websocket/rooms"
)

func init() {
	register("request_state", handleRequestState)
}

// handleRequestState replies to the requester with the current_state
// envelope(s) for whichever host:* rooms they are subscribed to. Used for
// resync after a network blip without leaving + rejoining rooms.
//
// M5 stage 5d narrowed this from "every running scraper" to "the requester's
// own host:* memberships", originally keyed on UserRooms(UserID) — which
// unioned every anonymous client's rooms under the empty user id. M31
// replaced that with the Event's per-sender Rooms capability (this
// connection's memberships only).
//   - host:<name> → JoinReplayForInstance(name) (one current_state envelope
//     carrying the runner's full instanceCache).
//   - host:all   → JoinReplayForHostAll() (one current_state with the full
//     hostsCache list).
//
// Auth: free for any connected client. Membership is the access gate —
// you only get state for rooms you've already successfully joined (the
// host RoomType's RequireAuth guard runs at join_room time).
func handleRequestState(e *Event) {
	if e.Services == nil || e.Services.Scraper == nil || e.Rooms == nil {
		return
	}
	for _, room := range e.Rooms() {
		switch {
		case room == rooms.HostAllRoom, room == rooms.SummaryRoom:
			for _, msg := range e.Services.Scraper.JoinReplayForHostAll() {
				e.SendRaw(msg)
			}
		case strings.HasPrefix(room, rooms.HostRoomPrefix+":"):
			rest := strings.TrimPrefix(room, rooms.HostRoomPrefix+":")
			parts := strings.SplitN(rest, ":", 2)
			name := parts[0]
			if len(parts) == 1 {
				for _, msg := range e.Services.Scraper.JoinReplayForInstance(name) {
					e.SendRaw(msg)
				}
			} else {
				class := parts[1]
				for _, msg := range e.Services.Scraper.JoinReplayForInstanceClass(name, class) {
					e.SendRaw(msg)
				}
			}
		}
	}
}
