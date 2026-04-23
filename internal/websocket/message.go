package websocket

import "encoding/json"

// Message type constants for routing.
const (
	TypeBroadcast = "broadcast"
	TypeRoom      = "room"
	TypeDirect    = "direct"
	TypeJoinRoom  = "join_room"
	TypeLeaveRoom = "leave_room"
	TypeError     = "error"
)

// Message is the wire format for all WebSocket communication.
// Hub inspects Type to decide routing. Payload is opaque project-specific data.
type Message struct {
	Type    string          `json:"type"`
	Room    string          `json:"room,omitempty"`
	Target  string          `json:"target,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}
