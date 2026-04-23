package websocket

// Service is the aggregate WebSocket interface.
// Implemented by websocket.Hub via structural typing.
type Service interface {
	Connected
	Rooms
	Broadcast
}
