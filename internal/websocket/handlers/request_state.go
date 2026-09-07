package handlers

import "github.com/Stewball32/xemu-cartographer/internal/authz"

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
// Auth: membership selects the rooms; each replay is then re-decided —
// authz.Can(scraper.state, Instance(name)) per instance room, and
// authz.Can(room.join, <that room>) for an aggregate feed, the same
// decision join_room made to admit it (host:all and host:summary are
// separate scope selectors, so neither stands in for the other) — so a
// principal whose access lapsed since it joined (the re-resolve tick
// evicts it from the room within a minute) gets nothing meanwhile.
func handleRequestState(e *Event) {
	if e.Services == nil || e.Services.Scraper == nil || e.Rooms == nil {
		return
	}
	for _, name := range e.Rooms() {
		room, err := authz.ParseRoom(name)
		if err != nil || !room.IsHost() {
			continue
		}
		switch {
		case room.IsHostAggregate():
			if !authz.Can(e.Authz, e.Principal, authz.ActionRoomJoin, authz.RoomRes(room)) {
				continue
			}
			for _, msg := range e.Services.Scraper.JoinReplayForHostAll() {
				e.SendRaw(msg)
			}
		default:
			if !authz.Can(e.Authz, e.Principal, authz.ActionScraperState, authz.Instance(room.Instance)) {
				continue
			}
			if room.Class == "" {
				for _, msg := range e.Services.Scraper.JoinReplayForInstance(room.Instance) {
					e.SendRaw(msg)
				}
			} else {
				for _, msg := range e.Services.Scraper.JoinReplayForInstanceClass(room.Instance, room.Class) {
					e.SendRaw(msg)
				}
			}
		}
	}
}
