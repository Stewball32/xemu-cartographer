package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/pocketbase/pocketbase/core"

	"github.com/xemu-cartographer/xemu-cartographer/internal/authz"
	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/pb/pbtest"
)

// tickInterval is the re-resolve period the connect-level tests run at.
const tickInterval = 20 * time.Millisecond

// waitFor polls cond until it holds or the deadline passes.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// hubClients snapshots the hub's client set.
func hubClients(h *Hub) []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]*Client, 0, len(h.clients))
	for c := range h.clients {
		out = append(out, c)
	}
	return out
}

// inRoom reports whether the client is a member of room.
func inRoom(h *Hub, c *Client, room string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.rooms[room][c]
}

// wsFixture is a Hub + NewHandler behind an httptest server, ticking at
// tickInterval. dial opens one client connection with the given query.
// Teardown runs in reverse registration order: client sockets close, every
// handler invocation is joined (NewHandler joins its own re-resolve loop,
// so no tick can touch the app after this), then the server, the hub and
// finally the PocketBase app go away.
type wsFixture struct {
	t   *testing.T
	app core.App
	hub *Hub
	srv *httptest.Server
}

func newWSFixture(t *testing.T, app core.App) *wsFixture {
	t.Helper()
	return newWSFixtureTicking(t, app, tickInterval)
}

// newWSFixtureTicking is newWSFixture with an explicit re-resolve period —
// pass something huge to pin behaviour the tick must not be credited for.
func newWSFixtureTicking(t *testing.T, app core.App, interval time.Duration) *wsFixture {
	t.Helper()
	hub := NewHub(app)
	go hub.Run()
	t.Cleanup(hub.Stop)

	restore := reResolveInterval
	reResolveInterval = interval
	t.Cleanup(func() { reResolveInterval = restore })

	handler := NewHandler(hub, app)
	var inflight sync.WaitGroup
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inflight.Add(1)
		defer inflight.Done()
		e := &core.RequestEvent{App: app}
		e.Response = w
		e.Request = r
		_ = handler(e)
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(inflight.Wait)
	return &wsFixture{t: t, app: app, hub: hub, srv: srv}
}

// dial connects with the query string (e.g. "token=…") and waits until the
// Hub has registered the connection, returning both ends.
func (f *wsFixture) dial(ctx context.Context, query string) (*websocket.Conn, *Client) {
	f.t.Helper()
	before := map[*Client]bool{}
	for _, c := range hubClients(f.hub) {
		before[c] = true
	}
	url := "ws" + strings.TrimPrefix(f.srv.URL, "http") + "/api/ws?" + query
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		f.t.Fatalf("dial %s: %v", url, err)
	}
	f.t.Cleanup(func() { _ = conn.CloseNow() })

	var client *Client
	waitFor(f.t, "hub registration", func() bool {
		for _, c := range hubClients(f.hub) {
			if !before[c] {
				client = c
				return true
			}
		}
		return false
	})
	return conn, client
}

// send writes one client→server message.
func send(ctx context.Context, t *testing.T, conn *websocket.Conn, msg Message) {
	t.Helper()
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("write %s: %v", data, err)
	}
}

// readFrame reads one server→client frame and decodes it with decodeFrame.
func readFrame(ctx context.Context, t *testing.T, conn *websocket.Conn) (Message, map[string]string) {
	t.Helper()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return decodeFrame(t, data)
}

// decodeFrame splits a frame into its envelope and the string-valued
// payload the Hub's own frames carry (error: code + message; room_left:
// reason). The payload map is empty for any other payload shape.
func decodeFrame(t *testing.T, data []byte) (Message, map[string]string) {
	t.Helper()
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("frame %s: %v", data, err)
	}
	payload := map[string]string{}
	if len(msg.Payload) > 0 {
		_ = json.Unmarshal(msg.Payload, &payload)
	}
	return msg, payload
}

// expectRoomLeft asserts the frame is room_left{room, reason:"forbidden"}.
func expectRoomLeft(t *testing.T, msg Message, payload map[string]string, room string) {
	t.Helper()
	if msg.Type != TypeRoomLeft || msg.Room != room || payload["reason"] != "forbidden" {
		t.Fatalf("frame %+v payload %v, want room_left room=%q reason=forbidden", msg, payload, room)
	}
}

// expectError asserts the frame is error{room, code}.
func expectError(t *testing.T, msg Message, payload map[string]string, room, code string) {
	t.Helper()
	if msg.Type != TypeError || msg.Room != room || payload["code"] != code || payload["message"] == "" {
		t.Fatalf("frame %+v payload %v, want error room=%q code=%q with a message", msg, payload, room, code)
	}
}

// TestReResolveTickEvicts is the W-2 eviction path end to end: a socket
// opened with a machine key resolves to that key's principal at connect;
// once the api_tokens row is revoked, the next re-resolve tick sends
// error{code:"session_revoked"} and closes the socket with 4401, and the
// Hub forgets the connection.
func TestReResolveTickEvicts(t *testing.T) {
	app, d := pbtest.NewApp(t)
	kid, token := pbtest.MintToken(t, app, d, "machine", []string{"room.join:*"})
	f := newWSFixture(t, app)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, client := f.dial(ctx, "token="+token)

	if p := client.Principal(); p.Kind != authz.KindMachine || p.ID != kid {
		t.Fatalf("connect resolved %+v, want machine %s", p, kid)
	}

	// The key stays valid across at least one tick: no frame, no close.
	time.Sleep(3 * tickInterval)
	if len(hubClients(f.hub)) != 1 {
		t.Fatal("valid key was dropped by the re-resolve tick")
	}

	rec := pbtest.TokenRecord(t, app, kid)
	pbtest.SetField(t, app, "api_tokens", rec.Id, "revoked", true)

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("expected the session_revoked frame before the close, got %v", err)
	}
	if code := errorCode(t, string(data)); code != "session_revoked" {
		t.Fatalf("frame %s: code %q, want session_revoked", data, code)
	}

	_, _, err = conn.Read(ctx)
	if err == nil {
		t.Fatal("socket still open after session_revoked")
	}
	if got := websocket.CloseStatus(err); got != closeSessionRevoked {
		t.Fatalf("close status %d (%v), want %d", got, err, closeSessionRevoked)
	}

	waitFor(t, "hub to forget the evicted client", func() bool { return len(hubClients(f.hub)) == 0 })
}

// TestReResolveTickLeavesRooms is the W-2 per-room re-check: a key whose
// scopes no longer admit it to a host room it joined is removed from that
// room on the next tick and told so with room_left{room, reason:forbidden}
// — but a credential that still resolves is not evicted, and the socket
// keeps working.
func TestReResolveTickLeavesRooms(t *testing.T) {
	app, d := pbtest.NewApp(t)
	kid, token := pbtest.MintToken(t, app, d, "machine", []string{"room.join:*"})
	f := newWSFixture(t, app)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, client := f.dial(ctx, "token="+token)

	send(ctx, t, conn, Message{Type: "join_room", Room: "host:summary"})
	waitFor(t, "join host:summary", func() bool { return inRoom(f.hub, client, "host:summary") })
	send(ctx, t, conn, Message{Type: "join_room", Room: "public:lobby"})
	waitFor(t, "join public:lobby", func() bool { return inRoom(f.hub, client, "public:lobby") })

	rec := pbtest.TokenRecord(t, app, kid)
	pbtest.SetField(t, app, "api_tokens", rec.Id, "scopes", []string{})

	waitFor(t, "re-resolve to leave host:summary", func() bool { return !inRoom(f.hub, client, "host:summary") })
	if !inRoom(f.hub, client, "public:lobby") {
		t.Fatal("re-resolve left a public room, which re-admits every kind")
	}
	if p := client.Principal(); len(p.Scopes) != 0 || p.Kind != authz.KindMachine {
		t.Fatalf("principal after re-resolve = %+v, want the same machine key with no scopes", p)
	}
	if len(hubClients(f.hub)) != 1 {
		t.Fatal("a key that still resolves was evicted")
	}

	// The client is told which room it lost, and only that one.
	msg, payload := readFrame(ctx, t, conn)
	expectRoomLeft(t, msg, payload, "host:summary")

	// The socket is still live: a fresh join is answered (forbidden, since
	// the scopes are gone) naming the room it refused, not dropped.
	send(ctx, t, conn, Message{Type: "join_room", Room: "host:all"})
	msg, payload = readFrame(ctx, t, conn)
	expectError(t, msg, payload, "host:all", "forbidden")
}

// TestReResolveTickLeavesAdminRoom: the room re-check is not limited to
// host: rooms — an admin JWT holds the admin room (and, through room.join:*,
// host:summary) only while the role still grants it; once the admin role's
// scopes are stripped the next tick removes it from both (one room_left
// frame each), keeps the public room, and the user, who still resolves,
// stays connected.
func TestReResolveTickLeavesAdminRoom(t *testing.T) {
	app, _ := pbtest.NewApp(t)
	user := pbtest.NewUser(t, app, "admin@example.com")
	pbtest.GrantRole(t, app, user.Id, "admin")
	token, err := user.NewAuthToken()
	if err != nil {
		t.Fatal(err)
	}
	f := newWSFixture(t, app)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, client := f.dial(ctx, "token="+token)

	if p := client.Principal(); p.Kind != authz.KindPBUser || p.UserID != user.Id {
		t.Fatalf("connect resolved %+v, want pb_user %s", p, user.Id)
	}
	for _, room := range []string{"admin", "host:summary", "public:lobby"} {
		send(ctx, t, conn, Message{Type: "join_room", Room: room})
		waitFor(t, "join "+room, func() bool { return inRoom(f.hub, client, room) })
	}

	pbtest.SetRoleScopes(t, app, "admin", []string{})

	waitFor(t, "re-resolve to leave the admin room", func() bool { return !inRoom(f.hub, client, "admin") })
	waitFor(t, "re-resolve to leave host:summary", func() bool { return !inRoom(f.hub, client, "host:summary") })
	if !inRoom(f.hub, client, "public:lobby") {
		t.Fatal("re-resolve left the public room")
	}
	if p := client.Principal(); p.Kind != authz.KindPBUser || len(p.Scopes) != 0 {
		t.Fatalf("principal after re-resolve = %+v, want the same user with no scopes", p)
	}
	if len(hubClients(f.hub)) != 1 {
		t.Fatal("a user that still resolves was evicted")
	}

	// One room_left per room lost, in the re-check's (map-random) order.
	left := map[string]bool{}
	for i := 0; i < 2; i++ {
		msg, payload := readFrame(ctx, t, conn)
		expectRoomLeft(t, msg, payload, msg.Room)
		left[msg.Room] = true
	}
	if !left["admin"] || !left["host:summary"] {
		t.Fatalf("room_left announced %v, want admin and host:summary", left)
	}

	send(ctx, t, conn, Message{Type: "join_room", Room: "admin"})
	msg, payload := readFrame(ctx, t, conn)
	expectError(t, msg, payload, "admin", "forbidden")
}

// TestConnectNobodyWithoutCredential: no ?token= / ?spectator= / ?console=
// resolves to Nobody — connected, able to join a public room, forbidden
// under host: — and a bad token is not an upgrade failure but the same
// Nobody socket (the "allow anonymous if not" convention).
func TestConnectNobodyWithoutCredential(t *testing.T) {
	app, _ := pbtest.NewApp(t)
	f := newWSFixture(t, app)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, query := range []string{"", "token=not.a.real.jwt"} {
		conn, client := f.dial(ctx, query)
		if p := client.Principal(); p.Kind != authz.KindAnonymous || p.BoundInstance() != "" || len(p.Scopes) != 0 {
			t.Fatalf("query %q resolved %+v, want Nobody", query, p)
		}
		send(ctx, t, conn, Message{Type: "join_room", Room: "public:lobby"})
		waitFor(t, "join public:lobby", func() bool { return inRoom(f.hub, client, "public:lobby") })

		send(ctx, t, conn, Message{Type: "join_room", Room: "host:summary"})
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("query %q: read: %v", query, err)
		}
		if code := errorCode(t, string(data)); code != "forbidden" {
			t.Fatalf("query %q: frame %s: code %q, want forbidden", query, data, code)
		}
		if inRoom(f.hub, client, "host:summary") {
			t.Fatalf("query %q: Nobody joined host:summary", query)
		}
	}
}
