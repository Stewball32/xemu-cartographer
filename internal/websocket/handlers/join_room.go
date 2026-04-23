package handlers

import "github.com/youruser/yourproject/internal/websocket/rooms"

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
}
