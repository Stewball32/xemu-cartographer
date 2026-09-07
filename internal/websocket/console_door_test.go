package websocket

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/authz/authztest"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb/pbtest"
	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
)

// consoleScraper is the scraperiface.Service the console-door tests hand the
// authz adapter: an instance list the test flips while the fixture's
// re-resolve tick and the Hub's join-time re-bind read it. pbtest.FakeScraper
// exposes bare slices, which would race with those goroutines under -race;
// this stub guards the list and is otherwise inert (the fixture wires no
// Services, so nothing else on the interface is reached).
type consoleScraper struct {
	scraperiface.Service
	mu    sync.Mutex
	infos []scraperiface.Info
}

// setInstances replaces the running-instance list.
func (s *consoleScraper) setInstances(infos ...scraperiface.Info) {
	s.mu.Lock()
	s.infos = append([]scraperiface.Info(nil), infos...)
	s.mu.Unlock()
}

func (s *consoleScraper) List() []scraperiface.Info {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]scraperiface.Info(nil), s.infos...)
}

func (s *consoleScraper) Membership() []scraperiface.ContainerMembership { return nil }

// box1 is the instance the console-door tests bind to: container xc-1 whose
// Xbox console name is BOX1.
var box1 = scraperiface.Info{Name: "xc-1", XboxName: "BOX1"}

// newConsoleApp is pbtest.NewApp over a consoleScraper.
func newConsoleApp(t *testing.T) (*consoleScraper, core.App) {
	t.Helper()
	scraper := &consoleScraper{}
	app, _ := pbtest.NewAppWith(t, scraper, func() string { return pbtest.DefaultPrefix })
	return scraper, app
}

// TestAdvConsoleDoorBeforeLive: an overlay that dials ?console=BOX1 before
// the runner has read that console's name connects unbound, and its join is
// refused with error{room, forbidden}. Once the instance is up, the very
// next join_room re-binds the connection on the spot (rebindConsole) and
// admits it — the tick is set to an hour so it cannot be what bound it. A
// class room of another instance stays forbidden: re-binding never widens
// past the one console the socket named.
func TestAdvConsoleDoorBeforeLive(t *testing.T) {
	scraper, app := newConsoleApp(t)
	f := newWSFixtureTicking(t, app, time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, client := f.dial(ctx, "console=BOX1")

	if p := client.Principal(); p.Kind != authz.KindAnonymous || p.BoundInstance() != "" || p.Extra["console"] != "BOX1" {
		t.Fatalf("connect resolved %+v, want the unbound BOX1 console principal", p)
	}

	send(ctx, t, conn, Message{Type: "join_room", Room: "host:xc-1:tick"})
	msg, payload := readFrame(ctx, t, conn)
	expectError(t, msg, payload, "host:xc-1:tick", "forbidden")
	if inRoom(f.hub, client, "host:xc-1:tick") {
		t.Fatal("unbound console joined an instance room")
	}

	scraper.setInstances(box1)

	send(ctx, t, conn, Message{Type: "join_room", Room: "host:xc-1:tick"})
	waitFor(t, "join after the console came live", func() bool { return inRoom(f.hub, client, "host:xc-1:tick") })
	if p := client.Principal(); p.BoundInstance() != "xc-1" || p.Extra["console"] != "BOX1" {
		t.Fatalf("principal after the join = %+v, want bound to xc-1 and still naming BOX1", p)
	}

	// Still fail-closed elsewhere: another instance, and the bare room.
	scraper.setInstances(box1, scraperiface.Info{Name: "xc-2", XboxName: "BOX2"})
	for _, room := range []string{"host:xc-2:tick", "host:xc-1"} {
		send(ctx, t, conn, Message{Type: "join_room", Room: room})
		msg, payload = readFrame(ctx, t, conn)
		expectError(t, msg, payload, room, "forbidden")
	}
}

// TestAdvConsoleDoorRestart: a bound, subscribed overlay whose container
// goes away is told room_left{room, forbidden} by the re-resolve tick (and
// dropped from the room) instead of going silently dark; when the console
// comes back, a plain re-join succeeds and the socket is live again.
func TestAdvConsoleDoorRestart(t *testing.T) {
	scraper, app := newConsoleApp(t)
	scraper.setInstances(box1)
	f := newWSFixture(t, app)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, client := f.dial(ctx, "console=BOX1")

	if p := client.Principal(); p.BoundInstance() != "xc-1" {
		t.Fatalf("connect resolved %+v, want bound to xc-1", p)
	}
	send(ctx, t, conn, Message{Type: "join_room", Room: "host:xc-1:tick"})
	waitFor(t, "join host:xc-1:tick", func() bool { return inRoom(f.hub, client, "host:xc-1:tick") })

	// Container restart: the runner (and the console name) disappears.
	scraper.setInstances()

	msg, payload := readFrame(ctx, t, conn)
	expectRoomLeft(t, msg, payload, "host:xc-1:tick")
	waitFor(t, "re-resolve to leave the room", func() bool { return !inRoom(f.hub, client, "host:xc-1:tick") })
	if p := client.Principal(); p.BoundInstance() != "" || p.Extra["console"] != "BOX1" {
		t.Fatalf("principal after the restart = %+v, want unbound and still naming BOX1", p)
	}
	if len(hubClients(f.hub)) != 1 {
		t.Fatal("an unbound console socket was evicted; it should wait for its console")
	}

	// The console is back: the re-join is admitted (by the join-time re-bind
	// or the tick, whichever runs first) and answered with nothing — the
	// next frame the client sees is the reply to the probe join after it.
	scraper.setInstances(box1)
	send(ctx, t, conn, Message{Type: "join_room", Room: "host:xc-1:tick"})
	waitFor(t, "re-join after the restart", func() bool { return inRoom(f.hub, client, "host:xc-1:tick") })
	send(ctx, t, conn, Message{Type: "join_room", Room: "nonsense"})
	msg, payload = readFrame(ctx, t, conn)
	expectError(t, msg, payload, "nonsense", "not_found")
}

// TestRebindConsoleMatchesReResolve pins rebindConsole to the binding the
// re-resolve tick makes (pb.ReResolve → consolePrincipal) for the same
// principal, so a join-time re-bind and a tick can never disagree, and
// pins what it leaves alone: a bound console (the tick's job to unbind),
// Nobody, and every non-anonymous kind.
func TestRebindConsoleMatchesReResolve(t *testing.T) {
	fake := &pbtest.FakeScraper{}
	app, d := pbtest.NewAppWith(t, fake, func() string { return pbtest.DefaultPrefix })
	h := NewHub(app) // deps unset: reads pb.Default(), as production does

	unbound := authz.Anonymous("", d.AnonymousScopes())
	unbound.Extra = map[string]string{"console": "BOX1"}
	c := addTestClient(h, unbound)

	// No such console yet: the principal stays unbound, exactly as the tick
	// would leave it.
	want, ok := pb.ReResolve(app, d, unbound)
	if got := h.rebindConsole(c); !ok || !reflect.DeepEqual(got, want) || got.BoundInstance() != "" {
		t.Fatalf("rebind before live = %+v, want the tick's %+v (unbound)", got, want)
	}

	fake.AddInstance("xc-1", "BOX1")
	want, ok = pb.ReResolve(app, d, unbound)
	got := h.rebindConsole(c)
	if !ok || !reflect.DeepEqual(got, want) || got.BoundInstance() != "xc-1" {
		t.Fatalf("rebind after live = %+v, want the tick's %+v (bound to xc-1)", got, want)
	}
	if !reflect.DeepEqual(c.Principal(), want) {
		t.Fatalf("client principal = %+v, want the re-bound %+v", c.Principal(), want)
	}

	// Bound already: returned as is, even once the instance is gone.
	fake.Infos = nil
	if got := h.rebindConsole(c); !reflect.DeepEqual(got, want) {
		t.Fatalf("rebind of a bound console = %+v, want it untouched %+v", got, want)
	}

	// Nothing to re-bind: Nobody and the other kinds pass through.
	for _, p := range []authz.Principal{authz.Nobody(), machineKey("k1"), userPrincipal("u1"), authz.Superuser("su")} {
		c := addTestClient(h, p)
		if got := h.rebindConsole(c); !reflect.DeepEqual(got, p) || !reflect.DeepEqual(c.Principal(), p) {
			t.Fatalf("rebind of %+v = %+v, want untouched", p, got)
		}
	}
}

// TestRecheckRoomsAnnouncesRoomLeft pins the server-side eviction frame:
// every room the re-check takes away is announced to that client alone with
// room_left{room, reason:forbidden}, queued before the membership goes; a
// room it still passes gets no frame.
func TestRecheckRoomsAnnouncesRoomLeft(t *testing.T) {
	h := NewHub(nil)
	h.deps = &authztest.FakeDeps{}
	c := addTestClient(h, machineKey("k1"), "host:summary", "host:pod-a:tick", "public:lobby")
	peer := addTestClient(h, machineKey("k2"), "host:summary")

	stripped := machineKey("k1")
	stripped.Scopes = nil
	c.setPrincipal(stripped)

	left := h.recheckRooms(c)
	if len(left) != 2 {
		t.Fatalf("left %v, want host:summary and host:pod-a:tick", left)
	}
	got := drainSend(c)
	if len(got) != 2 {
		t.Fatalf("client got %d frames %q, want one room_left per room lost", len(got), got)
	}
	rooms := map[string]bool{}
	for _, raw := range got {
		msg, payload := decodeFrame(t, []byte(raw))
		expectRoomLeft(t, msg, payload, msg.Room)
		rooms[msg.Room] = true
	}
	if !rooms["host:summary"] || !rooms["host:pod-a:tick"] {
		t.Fatalf("room_left announced %v, want host:summary and host:pod-a:tick", rooms)
	}
	if inRoom(h, c, "host:summary") || inRoom(h, c, "host:pod-a:tick") || !inRoom(h, c, "public:lobby") {
		t.Fatal("re-check left the wrong rooms")
	}
	if got := drainSend(peer); len(got) != 0 || !inRoom(h, peer, "host:summary") {
		t.Fatalf("peer got %q / left its room; the re-check is per client", got)
	}
}

// TestErrorFrameRoom pins the error frame's room field: a refused
// join_room / leave_room echoes the room the request named, whatever the
// code; every other error — the send whitelist, an unknown type — leaves
// it empty even when the request carried a room.
func TestErrorFrameRoom(t *testing.T) {
	tests := []struct {
		name     string
		p        authz.Principal
		msg      Message
		wantRoom string
		wantCode string
	}{
		{"join forbidden", authz.Nobody(), Message{Type: "join_room", Room: "host:summary"}, "host:summary", "forbidden"},
		{"join unknown type", authz.Nobody(), Message{Type: "join_room", Room: "nonsense"}, "nonsense", "not_found"},
		{"join bad name", authz.Nobody(), Message{Type: "join_room", Room: "host:"}, "host:", "bad_room"},
		{"unknown message type", machineKey("k1"), Message{Type: "no_such_type", Room: "public:lobby"}, "", "unknown_type"},
		{"whitelist refusal", authz.Nobody(), Message{Type: "request_state", Room: "public:lobby"}, "", "forbidden"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHub(nil)
			h.deps = &authztest.FakeDeps{}
			sender := addTestClient(h, tc.p)
			h.dispatch(incomingMsg{msg: tc.msg, sender: sender})
			got := drainSend(sender)
			if len(got) != 1 {
				t.Fatalf("got %d frames %q, want one error", len(got), got)
			}
			msg, payload := decodeFrame(t, []byte(got[0]))
			expectError(t, msg, payload, tc.wantRoom, tc.wantCode)
		})
	}
}
