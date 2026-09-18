package websocket

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/xemu-cartographer/xemu-cartographer/internal/authz"
	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/authztest"
	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/pb/pbtest"
)

// TestRemovedClientNeverPanicsOnSend is the deterministic half of the
// send-on-closed-channel fix: the Hub used to close a client's send channel
// on removal, so any sender still holding the pointer (a scraper runner's
// SendToRoomRaw, the re-resolve loop, a queued error) crashed the process
// with "send on closed channel". Now removal only marks the client closed;
// every send path drops for it, and a removed client can neither be put
// back in a room nor have a queued message handled on its behalf.
func TestRemovedClientNeverPanicsOnSend(t *testing.T) {
	h := NewHub(nil)
	h.deps = &authztest.FakeDeps{}
	c := addTestClient(h, userPrincipal("u1"), "public:lobby")
	peer := addTestClient(h, userPrincipal("u2"), "public:lobby")

	h.removeClient(c)
	h.removeClient(c) // idempotent
	if !c.closed() {
		t.Fatal("removed client not marked closed")
	}
	if connected(h, c) || inRoom(h, c, "public:lobby") {
		t.Fatal("removed client still indexed")
	}

	frame := []byte(`{"type":"broadcast"}`)
	h.trySend(c, frame)
	h.sendError(c, "", "forbidden", "x")
	h.sendRoomLeft(c, "public:lobby", "forbidden")
	h.BroadcastRaw(frame)
	h.SendToUserRaw("u1", frame)
	h.SendToRoomRaw("public:lobby", frame)
	h.broadcast(Message{Type: TypeBroadcast})
	h.sendToUser("u1", Message{Type: TypeDirect})
	h.sendToRoom("public:lobby", Message{Type: TypeRoom})
	if got := drainSend(c); len(got) != 0 {
		t.Fatalf("removed client was sent %q", got)
	}
	if got := drainSend(peer); len(got) != 4 {
		t.Fatalf("peer got %d frames %q, want the two broadcasts and two room sends", len(got), got)
	}

	// A join queued behind the removal cannot resurrect the client.
	h.addToRoom(c, "public:lobby")
	if inRoom(h, c, "public:lobby") {
		t.Fatal("addToRoom re-indexed a removed client")
	}
	h.dispatch(incomingMsg{msg: Message{Type: "join_room", Room: "public:lobby"}, sender: c})
	if inRoom(h, c, "public:lobby") {
		t.Fatal("dispatch joined a removed client")
	}
	if got := drainSend(c); len(got) != 0 {
		t.Fatalf("dispatch answered a removed client with %q", got)
	}
}

// TestFullBufferRemovesClient: a slow reader whose buffer is full has the
// frame dropped and is removed by Run (requestUnregister), after which it
// is marked closed — the non-blocking drop the Hub always did, minus the
// channel close that made it fatal.
func TestFullBufferRemovesClient(t *testing.T) {
	h := NewHub(nil)
	go h.Run()
	t.Cleanup(h.Stop)

	c := newClient(h, nil, authz.Nobody())
	h.register <- c
	waitFor(t, "registration", func() bool { return connected(h, c) })

	for i := 0; i < sendBufSize; i++ {
		h.trySend(c, []byte("fill"))
	}
	if !connected(h, c) {
		t.Fatal("client dropped before its buffer was full")
	}
	h.trySend(c, []byte("overflow"))
	waitFor(t, "Run to remove the slow client", func() bool { return !connected(h, c) })
	if !c.closed() {
		t.Fatal("removed client not marked closed")
	}
	h.trySend(c, []byte("late")) // no panic, no enqueue
	if got := drainSend(c); len(got) != sendBufSize {
		t.Fatalf("buffer holds %d frames, want exactly the %d that fit", len(got), sendBufSize)
	}
}

// TestRequestUnregisterAfterStop: once Run has exited nobody receives on
// unregister; a read loop ending then must return, not hang the handler.
func TestRequestUnregisterAfterStop(t *testing.T) {
	h := NewHub(nil)
	go h.Run()
	c := addTestClient(h, authz.Nobody())
	h.Stop()

	done := make(chan struct{})
	go func() {
		h.requestUnregister(c)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("requestUnregister blocked after Stop")
	}
}

// TestDisconnectRaceNoSendOnClosed is the reproduction that crashed the
// pre-fix Hub: a client writes a burst of frames and drops the socket while
// Run is still answering them (each frame is an unknown type, so each earns
// an error frame). With the send channel closed on unregister, the error
// enqueued for the just-removed client was a fatal "send on closed channel";
// -race and the iteration count make the window wide enough to hit.
func TestDisconnectRaceNoSendOnClosed(t *testing.T) {
	app, _ := pbtest.NewApp(t)
	f := newWSFixture(t, app)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(f.srv.URL, "http") + "/api/ws"
	burst, err := json.Marshal(Message{Type: "no_such_type"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		conn, _, err := websocket.Dial(ctx, url, nil)
		if err != nil {
			t.Fatalf("dial %d: %v", i, err)
		}
		for j := 0; j < 8; j++ {
			_ = conn.Write(ctx, websocket.MessageText, burst)
		}
		_ = conn.CloseNow()
	}
	waitFor(t, "hub to forget every dropped client", func() bool { return len(hubClients(f.hub)) == 0 })
}
