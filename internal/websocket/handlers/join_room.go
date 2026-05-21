package handlers

import (
	"strings"

	"github.com/Stewball32/xemu-cartographer/internal/websocket/rooms"
)

func init() {
	register("join_room", handleJoinRoom)
}

func handleJoinRoom(e *Event) {
	if e.Room == "" {
		return
	}

	rt, ok := rooms.Resolve(e.Room)
	if !ok {
		e.SendError("not_found", "unknown room type")
		return
	}

	if err := rt.CheckGuards(e.Services, e.User); err != nil {
		e.SendError("forbidden", err.Error())
		return
	}

	e.JoinRoom(e.Room)

	// Replay catch-up bytes for the joined room. Only host:* rooms have a
	// scraper-driven replay path; other rooms (admin, public) join silently.
	// rooms.Resolve already checked the type registry, but the "host:" prefix
	// distinguishes between the per-instance and aggregate flavours that
	// share the single host RoomType registration.
	if e.Services == nil || e.Services.Scraper == nil {
		return
	}
	switch {
	case e.Room == rooms.HostAllRoom, e.Room == rooms.SummaryRoom:
		for _, msg := range e.Services.Scraper.JoinReplayForHostAll() {
			e.SendRaw(msg)
		}
	case strings.HasPrefix(e.Room, rooms.HostRoomPrefix+":"):
		// "host:<rest>" where rest is either "<inst>" (legacy all-classes
		// replay) or "<inst>:<class>" (v2 per-class replay).
		rest := e.Room[len(rooms.HostRoomPrefix)+1:]
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
