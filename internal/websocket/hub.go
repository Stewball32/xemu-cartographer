package websocket

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/pocketbase/pocketbase/core"
	"github.com/youruser/yourproject/internal/guards"
	"github.com/youruser/yourproject/internal/websocket/handlers"
)

// Hub manages all connected WebSocket clients and rooms.
// State mutations are serialized through Run()'s select loop and protected
// by mu for concurrent reads from guards and resolvers.
type Hub struct {
	app      core.App
	services *guards.Services

	mu      sync.RWMutex
	clients map[*Client]bool
	users   map[string]map[*Client]bool
	rooms   map[string]map[*Client]bool

	register   chan *Client
	unregister chan *Client
	incoming   chan incomingMsg
	joinRoom   chan roomOp
	leaveRoom  chan roomOp

	done chan struct{}
	once sync.Once
}

// incomingMsg pairs a message with the client that sent it.
type incomingMsg struct {
	msg    Message
	sender *Client
}

// roomOp carries a client+room pair for join/leave operations.
type roomOp struct {
	client *Client
	room   string
}

var instance *Hub

// SetInstance stores the Hub for package-level access.
// Called from main.go after NewHub().
func SetInstance(h *Hub) { instance = h }

// Instance returns the Hub instance.
// Used by PocketBase hooks and Disgo handlers to send messages.
func Instance() *Hub { return instance }

// NewHub creates a Hub with initialized maps and channels.
func NewHub(app core.App) *Hub {
	return &Hub{
		app:        app,
		clients:    make(map[*Client]bool),
		users:      make(map[string]map[*Client]bool),
		rooms:      make(map[string]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		incoming:   make(chan incomingMsg, 256),
		joinRoom:   make(chan roomOp),
		leaveRoom:  make(chan roomOp),
		done:       make(chan struct{}),
	}
}

// SetServices stores the cross-system Services reference.
// Called from main.go after all systems are initialized.
func (h *Hub) SetServices(svc *guards.Services) {
	h.services = svc
}

// Run processes Hub channels in a loop. Start as a goroutine: go hub.Run().
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			if uid := client.UserID(); uid != "" {
				if h.users[uid] == nil {
					h.users[uid] = make(map[*Client]bool)
				}
				h.users[uid][client] = true
			}
			h.mu.Unlock()

		case client := <-h.unregister:
			h.removeClient(client)

		case im := <-h.incoming:
			// No lock held — dispatch calls handlers which may RLock
			// for guard checks, then Lock for JoinRoom/LeaveRoom.
			h.dispatch(im)

		case op := <-h.joinRoom:
			h.mu.Lock()
			if h.rooms[op.room] == nil {
				h.rooms[op.room] = make(map[*Client]bool)
			}
			h.rooms[op.room][op.client] = true
			h.mu.Unlock()

		case op := <-h.leaveRoom:
			h.mu.Lock()
			if members, ok := h.rooms[op.room]; ok {
				delete(members, op.client)
				if len(members) == 0 {
					delete(h.rooms, op.room)
				}
			}
			h.mu.Unlock()

		case <-h.done:
			h.mu.Lock()
			for client := range h.clients {
				close(client.send)
			}
			h.mu.Unlock()
			return
		}
	}
}

// dispatch routes an incoming message to registered handlers or default broadcast.
func (h *Hub) dispatch(im incomingMsg) {
	if handler, ok := handlers.Get(im.msg.Type); ok {
		handler(h.buildEvent(im))
		return
	}
	// No registered handler — default to broadcast.
	h.broadcast(im.msg)
}

// buildEvent constructs a handlers.Event with closures scoped to the sender.
func (h *Hub) buildEvent(im incomingMsg) *handlers.Event {
	return &handlers.Event{
		Services: h.services,
		App:      h.app,
		UserID:   im.sender.UserID(),
		User:     im.sender.user,
		Type:     im.msg.Type,
		Room:     im.msg.Room,
		Target:   im.msg.Target,
		Payload:  im.msg.Payload,
		Broadcast: func(payload json.RawMessage) {
			h.broadcast(Message{Type: im.msg.Type, Payload: payload})
		},
		SendToRoom: func(room string, payload json.RawMessage) {
			h.sendToRoom(room, Message{Type: im.msg.Type, Room: room, Payload: payload})
		},
		SendToUser: func(userID string, payload json.RawMessage) {
			h.sendToUser(userID, Message{Type: im.msg.Type, Target: userID, Payload: payload})
		},
		SendError: func(code string, message string) {
			errPayload, _ := json.Marshal(map[string]string{"code": code, "message": message})
			errMsg, _ := json.Marshal(Message{
				Type:    TypeError,
				Payload: errPayload,
			})
			h.trySend(im.sender, errMsg)
		},
		JoinRoom: func(room string) {
			h.mu.Lock()
			if h.rooms[room] == nil {
				h.rooms[room] = make(map[*Client]bool)
			}
			h.rooms[room][im.sender] = true
			h.mu.Unlock()
		},
		LeaveRoom: func(room string) {
			h.mu.Lock()
			if members, ok := h.rooms[room]; ok {
				delete(members, im.sender)
				if len(members) == 0 {
					delete(h.rooms, room)
				}
			}
			h.mu.Unlock()
		},
	}
}

// --- Public API (safe to call from any goroutine) ---

// Broadcast sends a message to all connected clients.
func (h *Hub) Broadcast(msg Message) {
	msg.Type = TypeBroadcast
	h.incoming <- incomingMsg{msg: msg}
}

// SendToUser sends a message to all connections for a given user ID.
func (h *Hub) SendToUser(userID string, msg Message) {
	msg.Type = TypeDirect
	msg.Target = userID
	h.incoming <- incomingMsg{msg: msg}
}

// SendToRoom sends a message to all clients in the given room.
func (h *Hub) SendToRoom(room string, msg Message) {
	msg.Type = TypeRoom
	msg.Room = room
	h.incoming <- incomingMsg{msg: msg}
}

// BroadcastRaw sends pre-marshaled bytes to all connected clients.
// Satisfies guards.WebSocketService.
func (h *Hub) BroadcastRaw(data []byte) {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.RUnlock()
	for _, client := range clients {
		h.trySend(client, data)
	}
}

// SendToUserRaw sends pre-marshaled bytes to all connections for a given user ID.
// Satisfies guards.WebSocketService.
func (h *Hub) SendToUserRaw(userID string, data []byte) {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.users[userID]))
	for client := range h.users[userID] {
		clients = append(clients, client)
	}
	h.mu.RUnlock()
	for _, client := range clients {
		h.trySend(client, data)
	}
}

// SendToRoomRaw sends pre-marshaled bytes to all clients in the given room.
// Satisfies guards.WebSocketService.
func (h *Hub) SendToRoomRaw(room string, data []byte) {
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.rooms[room]))
	for client := range h.rooms[room] {
		clients = append(clients, client)
	}
	h.mu.RUnlock()
	for _, client := range clients {
		h.trySend(client, data)
	}
}

// Stop signals the Run loop to exit and close all client connections.
func (h *Hub) Stop() {
	h.once.Do(func() { close(h.done) })
}

// --- Read methods (implement guards.WebSocketService) ---

// IsConnected reports whether a user has any active WebSocket connections.
func (h *Hub) IsConnected(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.users[userID]) > 0
}

// IsInRoom reports whether a user has a connection in the given room.
func (h *Hub) IsInRoom(userID string, room string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	members, ok := h.rooms[room]
	if !ok {
		return false
	}
	for client := range members {
		if client.UserID() == userID {
			return true
		}
	}
	return false
}

// UserRooms returns the rooms a user is currently in.
func (h *Hub) UserRooms(userID string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var result []string
	for room, members := range h.rooms {
		for client := range members {
			if client.UserID() == userID {
				result = append(result, room)
				break
			}
		}
	}
	return result
}

// --- Internal helpers (run on Run() goroutine only) ---

// broadcast sends data to every connected client.
func (h *Hub) broadcast(msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("ws: marshal error: %v", err)
		return
	}
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	h.mu.RUnlock()
	for _, client := range clients {
		h.trySend(client, data)
	}
}

// sendToUser sends data to all connections for a specific user.
func (h *Hub) sendToUser(userID string, msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("ws: marshal error: %v", err)
		return
	}
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.users[userID]))
	for client := range h.users[userID] {
		clients = append(clients, client)
	}
	h.mu.RUnlock()
	for _, client := range clients {
		h.trySend(client, data)
	}
}

// sendToRoom sends data to all clients in a room.
func (h *Hub) sendToRoom(room string, msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("ws: marshal error: %v", err)
		return
	}
	h.mu.RLock()
	clients := make([]*Client, 0, len(h.rooms[room]))
	for client := range h.rooms[room] {
		clients = append(clients, client)
	}
	h.mu.RUnlock()
	for _, client := range clients {
		h.trySend(client, data)
	}
}

// trySend attempts a non-blocking send. Schedules client removal if buffer full.
func (h *Hub) trySend(client *Client, data []byte) {
	select {
	case client.send <- data:
	default:
		// Defer removal to avoid lock contention — Run() handles unregister.
		go func() { h.unregister <- client }()
	}
}

// removeClient removes a client from all indexes and closes its send channel.
func (h *Hub) removeClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[client]; !ok {
		return
	}
	for room, members := range h.rooms {
		delete(members, client)
		if len(members) == 0 {
			delete(h.rooms, room)
		}
	}
	if uid := client.UserID(); uid != "" {
		delete(h.users[uid], client)
		if len(h.users[uid]) == 0 {
			delete(h.users, uid)
		}
	}
	delete(h.clients, client)
	close(client.send)
}
