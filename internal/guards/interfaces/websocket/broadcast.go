package websocket

// Broadcast abstracts sending pre-marshaled messages over WebSocket.
type Broadcast interface {
	BroadcastRaw(data []byte)
	SendToUserRaw(userID string, data []byte)
	SendToRoomRaw(room string, data []byte)
}
