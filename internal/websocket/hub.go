package websocket

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/coder/websocket"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb"
	"github.com/Stewball32/xemu-cartographer/internal/guards"
	"github.com/Stewball32/xemu-cartographer/internal/websocket/handlers"
)

// Hub manages all connected WebSocket clients and rooms.
// State mutations are serialized through Run()'s select loop and protected
// by mu for concurrent reads from guards and resolvers.
type Hub struct {
	app      core.App
	services *guards.Services
	// deps, when set, replaces pb.Default() as the authz.Deps handed to
	// handlers (Event.Authz) and to the re-resolve room re-check. Only
	// in-package tests set it (authztest.FakeDeps); production always reads
	// the process-wide adapter, which is nil — and therefore denies — until
	// main.go installs it.
	deps authz.Deps

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

	// Room occupancy observer (SetRoomObserver). Membership mutations post
	// the room name to roomEvents AFTER releasing mu (non-blocking; a full
	// channel sets dirty instead of dropping) and the observer goroutine
	// recomputes occupancy through RoomHasMembers before firing the hook —
	// so the hook never runs under mu and a lost post cannot leave an edge
	// unreported. observed is the observer goroutine's own record of the
	// rooms it last reported occupied (touched by nobody else).
	observer   atomic.Pointer[RoomObserver]
	roomEvents chan string
	dirty      atomic.Bool
	observed   map[string]bool
	obsOnce    sync.Once
}

// RoomObserver is the occupancy hook SetRoomObserver installs: occupied is
// true on a room's 0→1 edge and false on its 1→0 edge.
type RoomObserver func(room string, occupied bool)

// roomEventBuf bounds the posts queued for the observer goroutine; a burst
// beyond it (a mass disconnect) sets dirty and triggers a full re-walk.
const roomEventBuf = 1024

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
		roomEvents: make(chan string, roomEventBuf),
		observed:   make(map[string]bool),
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
			h.addToRoom(op.client, op.room)

		case op := <-h.leaveRoom:
			h.removeFromRoom(op.client, op.room)

		case <-h.done:
			h.mu.Lock()
			for client := range h.clients {
				client.markClosed()
			}
			h.mu.Unlock()
			return
		}
	}
}

// requestUnregister asks Run to remove the client. Safe from any goroutine
// and after Stop: once Run has exited nobody receives on unregister, and a
// readPump ending then (or a full-buffer drop) must not block forever.
func (h *Hub) requestUnregister(c *Client) {
	select {
	case h.unregister <- c:
	case <-h.done:
	}
}

// addToRoom joins a registered client to room. A client the Hub has already
// removed is refused so a join queued behind its unregister cannot resurrect
// it in the room index.
func (h *Hub) addToRoom(c *Client, room string) {
	h.mu.Lock()
	if !h.clients[c] {
		h.mu.Unlock()
		return
	}
	edge := h.rooms[room] == nil
	if edge {
		h.rooms[room] = make(map[*Client]bool)
	}
	h.rooms[room][c] = true
	h.mu.Unlock()
	if edge {
		h.noteRoomEdge(room)
	}
}

// removeFromRoom drops the client's membership of room (no-op if absent).
func (h *Hub) removeFromRoom(c *Client, room string) {
	h.mu.Lock()
	edge := false
	if members, ok := h.rooms[room]; ok {
		delete(members, c)
		if len(members) == 0 {
			delete(h.rooms, room)
			edge = true
		}
	}
	h.mu.Unlock()
	if edge {
		h.noteRoomEdge(room)
	}
}

// registered reports whether the Hub still holds the client.
func (h *Hub) registered(c *Client) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.clients[c]
}

// authzDeps returns the authz.Deps handlers decide against: the test
// override when set, else the process-wide adapter. A nil adapter is passed
// through as-is — authz.Can treats a nil (or typed-nil) Deps as "no_deps"
// and denies, so nothing is admitted before main.go installs it.
func (h *Hub) authzDeps() authz.Deps {
	if h.deps != nil {
		return h.deps
	}
	return pb.Default()
}

// dispatch routes an incoming message. Messages from a connection pass the
// per-kind send whitelist first, then go to their registered handler; a
// type with no handler is an error, never a broadcast (fail-closed, A.10).
// Messages with no sender come from the Hub's own Broadcast / SendToUser /
// SendToRoom API and are fanned out by their routing type.
func (h *Hub) dispatch(im incomingMsg) {
	if im.sender == nil {
		h.dispatchInternal(im.msg)
		return
	}

	// A message queued by a connection Run has since unregistered (its
	// readPump pushed the frame, then the disconnect) is dropped: there is
	// nobody to answer, and a handler must never act for a removed client.
	if !h.registered(im.sender) {
		return
	}

	// The whitelist runs BEFORE the handler lookup so a read-only kind
	// (spectator / device) or the console door (anonymous) can never reach a
	// request or control handler, whatever its own guards do. Denied senders
	// get an error frame and stay connected.
	if !authz.WSSendAllowed(im.sender.Principal().Kind, im.msg.Type) {
		h.sendError(im.sender, "", "forbidden", "message type not allowed for this connection")
		return
	}

	if handler, ok := handlers.Get(im.msg.Type); ok {
		handler(h.buildEvent(im))
		return
	}
	h.sendError(im.sender, "", "unknown_type", "no handler for message type "+strconv.Quote(im.msg.Type))
}

// dispatchInternal fans out a message the Hub's public API queued: no
// sender, no whitelist, no handler — just the routing its type names.
func (h *Hub) dispatchInternal(msg Message) {
	switch msg.Type {
	case TypeDirect:
		h.sendToUser(msg.Target, msg)
	case TypeRoom:
		h.sendToRoom(msg.Room, msg)
	default:
		h.broadcast(msg)
	}
}

// errorMessage marshals the error frame sent back to a client:
// {"type":"error","room":…,"payload":{"code":…,"message":…}}. room is the
// room the refused request named (join_room / leave_room) so the client can
// tell which subscription failed; every other error leaves it empty.
func errorMessage(room, code, message string) ([]byte, bool) {
	errPayload, err := json.Marshal(map[string]string{"code": code, "message": message})
	if err != nil {
		return nil, false
	}
	errMsg, err := json.Marshal(Message{Type: TypeError, Room: room, Payload: errPayload})
	if err != nil {
		return nil, false
	}
	return errMsg, true
}

// sendError enqueues an error frame for one client.
func (h *Hub) sendError(client *Client, room, code, message string) {
	if data, ok := errorMessage(room, code, message); ok {
		h.trySend(client, data)
	}
}

// roomLeftMessage marshals the frame that tells a client the server took a
// room away from it: {"type":"room_left","room":…,"payload":{"reason":…}}.
func roomLeftMessage(room, reason string) ([]byte, bool) {
	payload, err := json.Marshal(map[string]string{"reason": reason})
	if err != nil {
		return nil, false
	}
	msg, err := json.Marshal(Message{Type: TypeRoomLeft, Room: room, Payload: payload})
	if err != nil {
		return nil, false
	}
	return msg, true
}

// sendRoomLeft enqueues a room_left frame for one client.
func (h *Hub) sendRoomLeft(client *Client, room, reason string) {
	if data, ok := roomLeftMessage(room, reason); ok {
		h.trySend(client, data)
	}
}

// buildEvent constructs a handlers.Event with closures scoped to the sender.
func (h *Hub) buildEvent(im incomingMsg) *handlers.Event {
	principal := h.rebindConsole(im.sender)

	// Error frames answering join_room / leave_room echo the room the
	// request named so the client knows which subscription failed; every
	// other error keeps room empty.
	errRoom := ""
	if im.msg.Type == TypeJoinRoom || im.msg.Type == TypeLeaveRoom {
		errRoom = im.msg.Room
	}

	return &handlers.Event{
		Services:  h.services,
		App:       h.app,
		Authz:     h.authzDeps(),
		Principal: principal,
		UserID:    principal.UserID,
		Type:      im.msg.Type,
		Room:      im.msg.Room,
		Target:    im.msg.Target,
		Payload:   im.msg.Payload,
		Broadcast: func(payload json.RawMessage) {
			h.broadcast(Message{Type: im.msg.Type, Payload: payload})
		},
		SendToRoom: func(room string, payload json.RawMessage) {
			h.sendToRoom(room, Message{Type: im.msg.Type, Room: room, Payload: payload})
		},
		SendToUser: func(userID string, payload json.RawMessage) {
			h.sendToUser(userID, Message{Type: im.msg.Type, Target: userID, Payload: payload})
		},
		SendRaw: func(data []byte) {
			h.trySend(im.sender, data)
		},
		SendError: func(code string, message string) {
			h.sendError(im.sender, errRoom, code, message)
		},
		JoinRoom: func(room string) {
			h.addToRoom(im.sender, room)
		},
		LeaveRoom: func(room string) {
			h.removeFromRoom(im.sender, room)
		},
		Rooms: func() []string {
			return h.clientRooms(im.sender)
		},
	}
}

// rebindConsole refreshes the console-door binding of an anonymous
// connection that asked for a console (?console=<name>, kept in
// Extra["console"]) but is not bound to an instance yet — the overlay
// opened before the runner had read its XboxName, or the container is
// coming back from a restart — and returns the principal handlers decide
// with. Doing it here, on the message that needs it, means a join issued
// the moment the console comes live succeeds instead of waiting for the
// next re-resolve tick. The binding is the one pb.ReResolve makes on that
// tick (consolePrincipal: InstanceByConsole + AnonymousScopes), read
// through the Deps seam so a test hub answers the same way; a still-absent
// console leaves the principal unbound, and unbound joins nothing. Bound
// principals and every other kind are returned as they are.
func (h *Hub) rebindConsole(c *Client) authz.Principal {
	p := c.Principal()
	name := p.Extra["console"]
	if p.Kind != authz.KindAnonymous || p.BoundInstance() != "" || name == "" {
		return p
	}
	deps := h.authzDeps()
	if deps == nil {
		return p
	}
	next := authz.Anonymous(deps.InstanceByConsole(name), deps.AnonymousScopes())
	next.Extra = map[string]string{"console": name}
	c.setPrincipal(next)
	return next
}

// recheckRooms re-evaluates every room the client is joined to against its
// current principal and leaves the ones room.join no longer admits
// (re-resolve tick, W-2: role strip, roster loss after the grace window,
// console re-bound to another instance). W-2 names the host: rooms; the
// admin room is re-decided by the same call so an admin whose role was
// stripped stops receiving admin-room traffic at the next tick too, and a
// public room re-admits every kind, so nothing is lost by walking them all.
// Rooms that no longer parse are left too — a name that was joinable once
// must still be joinable now. Each room left is announced to the client
// with a room_left frame, queued before the membership goes so it precedes
// anything else the client sees about that room and the client can
// re-join once its access returns. Returns the rooms left, for the
// caller's log line.
func (h *Hub) recheckRooms(c *Client) []string {
	deps := h.authzDeps()
	p := c.Principal()
	var left []string
	for _, name := range h.clientRooms(c) {
		room, err := authz.ParseRoom(name)
		if err == nil && authz.Can(deps, p, authz.ActionRoomJoin, authz.RoomRes(room)) {
			continue
		}
		h.sendRoomLeft(c, name, "forbidden")
		h.removeFromRoom(c, name)
		left = append(left, name)
	}
	return left
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

// RoomHasMembers reports whether any client is currently joined to room.
// Used by the scraper manager's demand model to skip expensive per-tick
// reads (ReadTick / ReadGameData) when no client is subscribed to the
// per-class room that would consume them. Capture policies layer on top
// later (PRs 13–16) — this method is the WS-side input to that union.
func (h *Hub) RoomHasMembers(room string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms[room]) > 0
}

// clientRooms returns the rooms one specific connection is currently in.
// This is the per-SENDER view the handler Event's Rooms capability exposes;
// unlike UserRooms it never conflates two connections sharing a UserID
// (anonymous clients all carry "", and one user can have several tabs).
func (h *Hub) clientRooms(c *Client) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var result []string
	for room, members := range h.rooms {
		if members[c] {
			result = append(result, room)
		}
	}
	return result
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

// trySend attempts a non-blocking enqueue. Safe from any goroutine, before
// and after the client's removal: a removed client is skipped, and because
// send is never closed a frame that slips in between the check and the
// removal is merely left in a buffer nobody drains. A full buffer drops the
// frame and schedules the client's removal (a slow reader is cut off, not
// allowed to stall the Hub).
func (h *Hub) trySend(client *Client, data []byte) {
	if client.closed() {
		return
	}
	select {
	case client.send <- data:
	default:
		// Defer removal to avoid lock contention — Run() handles unregister.
		go h.requestUnregister(client)
	}
}

// removeClient removes a client from all indexes and signals its writePump
// to stop. Runs on the Run goroutine only; idempotent.
func (h *Hub) removeClient(client *Client) {
	h.mu.Lock()
	if _, ok := h.clients[client]; !ok {
		h.mu.Unlock()
		return
	}
	var emptied []string
	for room, members := range h.rooms {
		if !members[client] {
			continue
		}
		delete(members, client)
		if len(members) == 0 {
			delete(h.rooms, room)
			emptied = append(emptied, room)
		}
	}
	if uid := client.UserID(); uid != "" {
		delete(h.users[uid], client)
		if len(h.users[uid]) == 0 {
			delete(h.users, uid)
		}
	}
	delete(h.clients, client)
	client.markClosed()
	h.mu.Unlock()
	for _, room := range emptied {
		h.noteRoomEdge(room)
	}
}

// --- Room occupancy observer + eviction (DESIGN-STEP8 §8.3) ---

// SetRoomObserver installs fn as the occupancy hook: it is called with
// occupied=true when a room gains its first member and occupied=false when
// its last member leaves (removeFromRoom, or removeClient for every room the
// departing client emptied). fn runs on the Hub's observer goroutine, never
// under mu, and never twice in a row with the same value for one room: the
// goroutine recomputes occupancy through RoomHasMembers for every posted
// room rather than trusting an edge value, so a burst that overflows the
// post queue (dirty) is repaired by a re-walk of every room it reported
// occupied plus every room the Hub currently holds. A nil fn stops the
// notifications; the goroutine exits with Stop.
func (h *Hub) SetRoomObserver(fn RoomObserver) {
	if fn == nil {
		h.observer.Store(nil)
		return
	}
	h.observer.Store(&fn)
	h.obsOnce.Do(func() { go h.observeRooms() })
}

// noteRoomEdge posts a room whose occupancy just crossed an edge. Callers
// must NOT hold mu. Non-blocking: on overflow it sets dirty so the observer
// re-walks instead of silently losing the edge.
func (h *Hub) noteRoomEdge(room string) {
	if h.observer.Load() == nil {
		return
	}
	select {
	case h.roomEvents <- room:
	default:
		h.dirty.Store(true)
	}
}

// observeRooms is the observer goroutine: drain posts, recompute, fire.
func (h *Hub) observeRooms() {
	for {
		if h.dirty.Swap(false) {
			h.rewalkRooms()
		}
		select {
		case room := <-h.roomEvents:
			h.checkRoom(room)
		case <-h.done:
			return
		}
	}
}

// checkRoom recomputes one room's occupancy and fires the hook on change.
func (h *Hub) checkRoom(room string) {
	occupied := h.RoomHasMembers(room)
	if h.observed[room] == occupied {
		return
	}
	if occupied {
		h.observed[room] = true
	} else {
		delete(h.observed, room)
	}
	if fn := h.observer.Load(); fn != nil {
		(*fn)(room, occupied)
	}
}

// rewalkRooms re-checks every room the observer reported occupied (a lost
// 1→0 edge) and every room the Hub currently holds (a lost 0→1 edge).
func (h *Hub) rewalkRooms() {
	rooms := make(map[string]bool, len(h.observed))
	for room := range h.observed {
		rooms[room] = true
	}
	h.mu.RLock()
	for room := range h.rooms {
		rooms[room] = true
	}
	h.mu.RUnlock()
	for room := range rooms {
		h.checkRoom(room)
	}
}

// EvictRoomPrefix closes the socket of every client holding a room that is
// prefix itself or a ":"-separated descendant of it ("host:box1" matches
// "host:box1" and "host:box1:tick", never "host:box10"; "host:" / "host"
// match the whole family) — the wire-mode consumer's "upstream resync"
// (1012 + reason, no error frame: wire.md has no resync code, and the
// frontend reconnects on any close it did not see session_revoked before).
// Each close runs on its own goroutine so a stalled peer cannot hold the
// others up; the client leaves the Hub when its readPump ends, as with any
// disconnect. Returns the number of clients evicted.
func (h *Hub) EvictRoomPrefix(prefix string, status websocket.StatusCode, reason string) int {
	victims := map[*Client]bool{}
	bare := strings.TrimSuffix(prefix, ":")
	h.mu.RLock()
	for room, members := range h.rooms {
		if room != bare && !strings.HasPrefix(room, bare+":") {
			continue
		}
		for c := range members {
			victims[c] = true
		}
	}
	h.mu.RUnlock()
	for c := range victims {
		go c.evictWith(context.Background(), status, "", reason)
	}
	return len(victims)
}
