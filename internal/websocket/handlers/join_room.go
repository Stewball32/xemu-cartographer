package handlers

import (
	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/websocket/rooms"
)

func init() {
	register("join_room", handleJoinRoom)
}

// handleJoinRoom admits the sender to a room in three steps: the room type
// must be registered (rooms.Resolve — "not_found"), the name must parse
// (authz.ParseRoom — "bad_room"), and authz.Can(room.join) must admit the
// principal ("forbidden"). That single Can call replaces the old guard walk,
// host ladder and console door (DESIGN-STEP6 §4): rostered members reach
// their bare host:<inst> room, the admin room needs admin.admin, the
// aggregate feeds need a room.join scope, and the bound kinds (spectator /
// device / the anonymous console door) reach only class rooms of the one
// instance they are bound to, when a scope names that class.
func handleJoinRoom(e *Event) {
	if e.Room == "" {
		return
	}

	if _, ok := rooms.Resolve(e.Room); !ok {
		e.SendError("not_found", "unknown room type")
		return
	}

	room, err := authz.ParseRoom(e.Room)
	if err != nil {
		e.SendError("bad_room", err.Error())
		return
	}

	if !authz.Can(e.Authz, e.Principal, authz.ActionRoomJoin, authz.RoomRes(room)) {
		e.SendError("forbidden", "not allowed to join this room")
		return
	}

	e.JoinRoom(e.Room)

	// Replay catch-up bytes for the joined room. Only host:* rooms have a
	// scraper-driven replay path; other rooms (admin, public) join silently.
	// The parsed Room distinguishes the aggregate feeds from the per-instance
	// flavours that share the single host RoomType registration.
	if e.Services == nil || e.Services.Scraper == nil || !room.IsHost() {
		return
	}
	switch {
	case room.IsHostAggregate():
		for _, msg := range e.Services.Scraper.JoinReplayForHostAll() {
			e.SendRaw(msg)
		}
	case room.Class == "":
		// Bare "host:<inst>": legacy all-classes replay.
		for _, msg := range e.Services.Scraper.JoinReplayForInstance(room.Instance) {
			e.SendRaw(msg)
		}
	default:
		// "host:<inst>:<class>": v2 per-class replay.
		for _, msg := range e.Services.Scraper.JoinReplayForInstanceClass(room.Instance, room.Class) {
			e.SendRaw(msg)
		}
	}
}
