package handlers

import (
	"encoding/json"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/guards"
)

// Event is passed to handlers when a WebSocket message arrives.
// Response capabilities are closures set by the Hub before dispatch,
// avoiding import cycles between handlers and the parent websocket package.
type Event struct {
	Services *guards.Services // Cross-system access for guards and resolvers.
	App      core.App         // PocketBase app for DB queries in guards/handlers.
	// Authz is the authz.Deps every Can call in a handler decides against
	// (the process-wide pb adapter in production, authztest.FakeDeps in
	// tests). A nil Authz denies everything — authz.Can never panics on it.
	Authz authz.Deps
	// Principal is who the sending connection is, as resolved at connect
	// (pb.ResolveWS) and refreshed on the re-resolve tick. It is the only
	// identity a handler sees: kind, scopes, roles and the bound instance
	// all live here, and access is asked as authz.Can(Authz, Principal, …).
	Principal authz.Principal
	UserID    string          // Principal.UserID; "" when not a users record.
	Type      string          // Message type that triggered this handler.
	Room      string          // Target room (if applicable).
	Target    string          // Target user ID (if applicable).
	Payload   json.RawMessage // Opaque project-specific data.

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
