package handlers

import (
	"encoding/json"

	"github.com/Stewball32/xemu-cartographer/internal/guards"
	"github.com/pocketbase/pocketbase/core"
)

// Event is passed to handlers when a WebSocket message arrives.
// Response capabilities are closures set by the Hub before dispatch,
// avoiding import cycles between handlers and the parent websocket package.
type Event struct {
	Services *guards.Services // Cross-system access for guards and resolvers.
	App      core.App         // PocketBase app for DB queries in guards/handlers.
	UserID   string           // Authenticated user ID, "" for anonymous.
	User     *core.Record     // Full PocketBase user record, nil for anonymous.
	// ConsoleName is a tokenless console-overlay connection's target console
	// (from ?console=<name>); "" for normal connections. join_room admits such a
	// client to the host:<instance> room currently rostering that console.
	ConsoleName string
	Type        string          // Message type that triggered this handler.
	Room        string          // Target room (if applicable).
	Target      string          // Target user ID (if applicable).
	Payload     json.RawMessage // Opaque project-specific data.

	// Response capabilities (set by Hub before dispatch).
	Broadcast  func(msg json.RawMessage)
	SendToRoom func(room string, msg json.RawMessage)
	SendToUser func(userID string, msg json.RawMessage)
	SendRaw    func(data []byte)                 // Send pre-marshaled bytes back to the sender.
	SendError  func(code string, message string) // Send error back to sender.
	JoinRoom   func(room string)
	LeaveRoom  func(room string)
	// Rooms returns the rooms THIS connection is currently in. Per-sender, not
	// per-user: keying resync/replay on UserRooms(UserID) would union every
	// anonymous client (all share UserID "") and every tab of the same user.
	Rooms func() []string
}

// HandlerFunc processes a WebSocket event.
type HandlerFunc func(e *Event)

var registry = map[string]HandlerFunc{}

// register adds a handler for a message type. Called from init() in handler files.
func register(msgType string, h HandlerFunc) {
	registry[msgType] = h
}

// Get returns the handler registered for the given message type.
func Get(msgType string) (HandlerFunc, bool) {
	h, ok := registry[msgType]
	return h, ok
}
