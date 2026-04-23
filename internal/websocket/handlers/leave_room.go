package handlers

func init() {
	register("leave_room", handleLeaveRoom)
}

func handleLeaveRoom(e *Event) {
	if e.Room != "" {
		e.LeaveRoom(e.Room)
	}
}
