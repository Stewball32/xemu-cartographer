package websocket

// Rooms abstracts WebSocket room membership queries.
type Rooms interface {
	IsInRoom(userID string, room string) bool
	UserRooms(userID string) []string
}
