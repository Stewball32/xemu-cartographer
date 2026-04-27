package handlers

import "github.com/Stewball32/xemu-cartographer/internal/websocket/rooms"

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

	// Mid-match overlay subscribers don't otherwise receive a snapshot until
	// the next game-state transition; replay the latest cached snapshot for
	// each running scraper so the UI can render immediately.
	if rt.Name == "overlay" && e.Services != nil && e.Services.Scraper != nil {
		for _, snap := range e.Services.Scraper.LatestSnapshotMessages() {
			e.SendRaw(snap)
		}
	}
}
