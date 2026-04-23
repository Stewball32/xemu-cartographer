package websocket

// Connected abstracts WebSocket connection state queries.
type Connected interface {
	IsConnected(userID string) bool
}
