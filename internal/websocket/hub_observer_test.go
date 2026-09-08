package websocket

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/Stewball32/xemu-cartographer/internal/authz/pb/pbtest"
)

// occupancyEdge is one observer callback.
type occupancyEdge struct {
	room     string
	occupied bool
}

// roomRecorder collects observer callbacks and, on every call, proves the
// hook is not running under the Hub's mutex: a write lock taken from a
// helper goroutine inside the callback must go through promptly — a
// concurrent mutator releases mu within microseconds, whereas a hook fired
// under mu (on the mutator's own goroutine) would hold it for as long as
// the callback runs, i.e. past the probe's deadline.
type roomRecorder struct {
	mu        sync.Mutex
	edges     []occupancyEdge
	underLock int
	gate      chan struct{} // when non-nil, callbacks block on it
	gated     bool
}

func newRoomRecorder(h *Hub) *roomRecorder {
	r := &roomRecorder{}
	h.SetRoomObserver(func(room string, occupied bool) {
		locked := make(chan struct{})
		go func() {
			h.mu.Lock()
			h.mu.Unlock()
			close(locked)
		}()
		select {
		case <-locked:
		case <-time.After(2 * time.Second):
			r.mu.Lock()
			r.underLock++
			r.mu.Unlock()
		}
		r.mu.Lock()
		r.edges = append(r.edges, occupancyEdge{room, occupied})
		gate, gated := r.gate, r.gated
		r.mu.Unlock()
		if gated {
			<-gate
		}
	})
	return r
}

// setGate makes every subsequent callback block until release.
func (r *roomRecorder) setGate() (release func()) {
	r.mu.Lock()
	r.gate = make(chan struct{})
	r.gated = true
	gate := r.gate
	r.mu.Unlock()
	var once sync.Once
	return func() { once.Do(func() { close(gate) }) }
}

func (r *roomRecorder) snapshot() []occupancyEdge {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]occupancyEdge(nil), r.edges...)
}

func (r *roomRecorder) forRoom(room string) []bool {
	var out []bool
	for _, e := range r.snapshot() {
		if e.room == room {
			out = append(out, e.occupied)
		}
	}
	return out
}

func (r *roomRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.edges)
}

func (r *roomRecorder) lockViolations() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.underLock
}

// expectEdges waits until the recorder holds exactly want, then settles and
// checks nothing else arrived.
func expectEdges(t *testing.T, r *roomRecorder, want []occupancyEdge) {
	t.Helper()
	waitFor(t, fmt.Sprintf("%d observer edges", len(want)), func() bool { return r.count() >= len(want) })
	time.Sleep(20 * time.Millisecond)
	got := r.snapshot()
	if len(got) != len(want) {
		t.Fatalf("observer edges = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("observer edge %d = %v, want %v (all %v)", i, got[i], want[i], got)
		}
	}
	if n := r.lockViolations(); n != 0 {
		t.Fatalf("observer fired %d times under h.mu", n)
	}
}

// TestRoomObserverEdges: the hook fires on the 0→1 and 1→0 edges only —
// second joiner, first leaver and a client that is already a member are
// silent — never under the Hub's mutex, and a room re-occupied after
// emptying reports a fresh 0→1.
func TestRoomObserverEdges(t *testing.T) {
	h := NewHub(nil)
	t.Cleanup(h.Stop)
	rec := newRoomRecorder(h)
	const room = "host:pod-a:tick"

	c1 := addTestClient(h, machineKey("k1"))
	c2 := addTestClient(h, machineKey("k2"))

	h.addToRoom(c1, room)
	expectEdges(t, rec, []occupancyEdge{{room, true}})

	h.addToRoom(c2, room)
	h.addToRoom(c1, room) // already a member
	h.removeFromRoom(c1, room)
	expectEdges(t, rec, []occupancyEdge{{room, true}})

	h.removeFromRoom(c2, room)
	expectEdges(t, rec, []occupancyEdge{{room, true}, {room, false}})

	h.removeFromRoom(c2, room) // absent: no edge
	h.addToRoom(c1, room)
	expectEdges(t, rec, []occupancyEdge{{room, true}, {room, false}, {room, true}})

	// A client the Hub already removed cannot re-occupy a room.
	h.removeClient(c1)
	h.addToRoom(c1, room)
	expectEdges(t, rec, []occupancyEdge{{room, true}, {room, false}, {room, true}, {room, false}})
}

// TestRoomObserverNilBeforeSet: without an observer membership churn is
// silent and cheap (no goroutine, nothing queued), and SetRoomObserver(nil)
// switches the notifications back off.
func TestRoomObserverNilBeforeSet(t *testing.T) {
	h := NewHub(nil)
	t.Cleanup(h.Stop)
	c := addTestClient(h, machineKey("k1"))
	h.addToRoom(c, "host:pod-a:tick")
	h.removeFromRoom(c, "host:pod-a:tick")
	if n := len(h.roomEvents); n != 0 {
		t.Fatalf("%d posts queued with no observer installed", n)
	}

	rec := newRoomRecorder(h)
	h.addToRoom(c, "host:pod-a:tick")
	expectEdges(t, rec, []occupancyEdge{{"host:pod-a:tick", true}})

	h.SetRoomObserver(nil)
	h.removeFromRoom(c, "host:pod-a:tick")
	time.Sleep(20 * time.Millisecond)
	if got := rec.snapshot(); len(got) != 1 {
		t.Fatalf("edges after SetRoomObserver(nil) = %v, want the one before it", got)
	}
}

// TestRoomObserverRemoveClientFiresAllRooms: a disconnect empties every
// room the client was alone in, and each one is reported; a room another
// client still holds is not.
func TestRoomObserverRemoveClientFiresAllRooms(t *testing.T) {
	h := NewHub(nil)
	t.Cleanup(h.Stop)
	rec := newRoomRecorder(h)

	c := addTestClient(h, machineKey("k1"))
	other := addTestClient(h, machineKey("k2"))
	for _, room := range []string{"host:pod-a:tick", "host:pod-a:objects", "host:summary"} {
		h.addToRoom(c, room)
	}
	h.addToRoom(other, "host:summary")
	waitFor(t, "three 0→1 edges", func() bool { return rec.count() == 3 })

	h.removeClient(c)
	waitFor(t, "two 1→0 edges", func() bool { return rec.count() == 5 })
	time.Sleep(20 * time.Millisecond)

	for _, room := range []string{"host:pod-a:tick", "host:pod-a:objects"} {
		if got := rec.forRoom(room); len(got) != 2 || !got[0] || got[1] {
			t.Errorf("%s edges = %v, want [true false]", room, got)
		}
	}
	if got := rec.forRoom("host:summary"); len(got) != 1 || !got[0] {
		t.Errorf("host:summary edges = %v, want [true] (other still holds it)", got)
	}
	if n := rec.lockViolations(); n != 0 {
		t.Fatalf("observer fired %d times under h.mu", n)
	}
}

// TestObserverBurstDoesNotLeakSubscription (§13): 2000 clients share one
// firehose room and each holds a private one; they all disconnect at once
// through the Run loop while the observer is stalled inside a callback, so
// the 2001 emptied-room posts overflow the 1024-deep queue and the dirty
// re-walk has to repair what the queue lost. The firehose room must end
// occupied=false exactly once and no room may stay reported occupied.
func TestObserverBurstDoesNotLeakSubscription(t *testing.T) {
	h := NewHub(nil)
	go h.Run()
	t.Cleanup(h.Stop)
	rec := newRoomRecorder(h)
	const n = 2000
	const firehose = "host:pod-a:tick"

	clients := make([]*Client, n)
	for i := range clients {
		clients[i] = addTestClient(h, machineKey(fmt.Sprintf("k%d", i)))
		h.addToRoom(clients[i], firehose)
		h.addToRoom(clients[i], fmt.Sprintf("host:pod-a:dbg%d", i))
	}
	waitFor(t, "all 0→1 edges observed", func() bool { return rec.count() == n+1 })
	if got := rec.forRoom(firehose); len(got) != 1 || !got[0] {
		t.Fatalf("firehose edges before the burst = %v, want [true]", got)
	}

	release := rec.setGate()
	defer release()
	// Stall the observer on a fresh edge so every post below queues.
	sentinel := addTestClient(h, machineKey("sentinel"))
	h.addToRoom(sentinel, "host:pod-b:tick")
	waitFor(t, "observer stalled in the callback", func() bool { return rec.count() == n+2 })

	var wg sync.WaitGroup
	for _, c := range clients {
		wg.Add(1)
		go func(c *Client) {
			defer wg.Done()
			h.requestUnregister(c)
		}(c)
	}
	wg.Wait()
	waitFor(t, "Run to drop every client", func() bool { return len(hubClients(h)) == 1 })
	if !h.dirty.Load() {
		t.Fatal("2001 posts against a stalled observer did not overflow the queue")
	}

	release()
	waitFor(t, "firehose 1→0 edge", func() bool {
		got := rec.forRoom(firehose)
		return len(got) >= 2 && !got[len(got)-1]
	})
	waitFor(t, "observer to drain", func() bool { return len(h.roomEvents) == 0 && !h.dirty.Load() })
	time.Sleep(50 * time.Millisecond)

	if got := rec.forRoom(firehose); len(got) != 2 || !got[0] || got[1] {
		t.Fatalf("firehose edges = %v, want exactly [true false]", got)
	}
	last := map[string]bool{}
	for _, e := range rec.snapshot() {
		last[e.room] = e.occupied
	}
	for room, occ := range last {
		if occ && room != "host:pod-b:tick" {
			t.Errorf("%s still reported occupied after the burst", room)
		}
	}
	if !last["host:pod-b:tick"] {
		t.Error("sentinel room lost its occupied=true")
	}
	if n := rec.lockViolations(); n != 0 {
		t.Fatalf("observer fired %d times under h.mu", n)
	}
}

// TestEvictRoomPrefix: every client holding a room under the prefix is
// closed with the given status and reason and NO error frame (the 1012
// resync path — wire.md has no resync code); clients elsewhere stay, and
// readPump's exit unregisters the evicted one like any disconnect.
func TestEvictRoomPrefix(t *testing.T) {
	app, d := pbtest.NewApp(t)
	_, token := pbtest.MintToken(t, app, d, "machine", []string{"room.join:*"})
	f := newWSFixtureTicking(t, app, time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	connA, clientA := f.dial(ctx, "token="+token)
	connB, clientB := f.dial(ctx, "token="+token)
	f.hub.addToRoom(clientA, "host:pod-a:tick")
	f.hub.addToRoom(clientB, "admin:dashboard")

	if got := f.hub.EvictRoomPrefix("host:", websocket.StatusServiceRestart, "upstream resync"); got != 1 {
		t.Fatalf("EvictRoomPrefix evicted %d clients, want 1", got)
	}

	_, data, err := connA.Read(ctx)
	if err == nil {
		t.Fatalf("evicted client got a frame before the close: %s", data)
	}
	if got := websocket.CloseStatus(err); got != websocket.StatusServiceRestart {
		t.Fatalf("close status %d (%v), want 1012", got, err)
	}
	var ce websocket.CloseError
	if !errors.As(err, &ce) || ce.Reason != "upstream resync" {
		t.Fatalf("close error %v, want reason \"upstream resync\"", err)
	}
	waitFor(t, "hub to forget the evicted client", func() bool { return !connected(f.hub, clientA) })
	if !connected(f.hub, clientB) {
		t.Fatal("client outside the prefix was dropped")
	}

	// B's socket still round-trips: an unknown type is answered with an
	// error frame, not a close.
	send(ctx, t, connB, Message{Type: "nope"})
	if msg, _ := readFrame(ctx, t, connB); msg.Type != TypeError {
		t.Fatalf("untouched client got %+v, want an error frame", msg)
	}

	if got := f.hub.EvictRoomPrefix("host:", websocket.StatusServiceRestart, "again"); got != 0 {
		t.Fatalf("second EvictRoomPrefix evicted %d, want 0", got)
	}
}
