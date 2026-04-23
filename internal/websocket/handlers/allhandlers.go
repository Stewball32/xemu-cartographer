package handlers

import (
	"encoding/json"

	"github.com/pocketbase/pocketbase/core"
	"github.com/youruser/yourproject/internal/guards"
)

// Event is passed to handlers when a WebSocket message arrives.
// Response capabilities are closures set by the Hub before dispatch,
// avoiding import cycles between handlers and the parent websocket package.
type Event struct {
	Services *guards.Services // Cross-system access for guards and resolvers.
	App      core.App         // PocketBase app for DB queries in guards/handlers.
	UserID  string          // Authenticated user ID, "" for anonymous.
	User    *core.Record    // Full PocketBase user record, nil for anonymous.
	Type    string          // Message type that triggered this handler.
	Room    string          // Target room (if applicable).
	Target  string          // Target user ID (if applicable).
	Payload json.RawMessage // Opaque project-specific data.

	// Response capabilities (set by Hub before dispatch).
	Broadcast  func(msg json.RawMessage)
	SendToRoom func(room string, msg json.RawMessage)
	SendToUser func(userID string, msg json.RawMessage)
	SendError  func(code string, message string) // Send error back to sender.
	JoinRoom   func(room string)
	LeaveRoom  func(room string)
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
