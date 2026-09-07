package leaguescraper

import (
	"log"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/guards"
	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
	"github.com/xemu-cartographer/xc-scraper/runner"
	"github.com/xemu-cartographer/xc-scraper/wire"
)

// WireAdapter is the league server's view of the scraper manager: it embeds
// *runner.Manager (so Start/Stop/List/Inspect/InstanceState/Membership and
// every setter pass straight through) and satisfies scraperiface.Service by
// framing the manager's request/reply envelopes (runner.Reply) as the
// wire.Message{Type:"scraper", Room:…} bytes the WebSocket handlers hand to a
// client. Since step 7 part 3c the manager itself returns bare envelopes and
// knows nothing about rooms, principals or the hub; everything transport- or
// league-shaped lives here.
//
// Room mapping (must stay byte-for-byte what the manager framed before 3c —
// manager runner.go wrapRoomMessage/marshalRoomMessage, aggregator.go
// joinReplay, events.go EventsReply, probe.go ProbeReply, hello.go):
//
//	class "summary" (instance "")   → wire.SummaryRoom             ("host:summary")
//	class "events" / "probe"        → wire.RoomForInstance         ("host:<inst>", the
//	                                  legacy per-instance room the request/reply
//	                                  channels always used — protocol bug 4, preserved)
//	every other class               → wire.RoomForInstanceClass    ("host:<inst>:<class>")
//	hello                           → no room (connection-scoped handshake)
//
// svc is retained for future league-side needs (the hub, the app); the
// reply paths themselves only need the manager.
type WireAdapter struct {
	*runner.Manager
	svc *guards.Services
}

// Compile-time proof that the adapter is what main.go stores in
// guards.Services.Scraper / hands to authzpb.NewDeps.
var _ scraperiface.Service = (*WireAdapter)(nil)

// NewWireAdapter wraps m for the league server. svc may be nil in tests.
func NewWireAdapter(m *runner.Manager, svc *guards.Services) *WireAdapter {
	return &WireAdapter{Manager: m, svc: svc}
}

// JoinReplayMessages frames every runner's per-class replay envelopes for
// their host:<name>:<class> rooms (scraperiface.JoinReplay).
func (a *WireAdapter) JoinReplayMessages() [][]byte {
	return frameReplies(a.Manager.JoinReplayMessages())
}

// JoinReplayForInstance frames one runner's per-class replay envelopes for
// their host:<name>:<class> rooms; nil when the runner does not exist.
func (a *WireAdapter) JoinReplayForInstance(name string) [][]byte {
	return frameReplies(a.Manager.JoinReplayForInstance(name))
}

// JoinReplayForInstanceClass frames the single cached envelope for
// host:<name>:<class>; nil when the runner or class has no current data.
func (a *WireAdapter) JoinReplayForInstanceClass(name, class string) [][]byte {
	return frameReplies(a.Manager.JoinReplayForInstanceClass(name, class))
}

// JoinReplayForHostAll frames the current summary envelope for
// wire.SummaryRoom.
func (a *WireAdapter) JoinReplayForHostAll() [][]byte {
	return frameReplies(a.Manager.JoinReplayForHostAll())
}

// EventsReply frames the request_events reply envelope for the legacy
// host:<instance> room (scraperiface.EventsReply). (nil, false) when the
// manager has no runner for instance or framing fails.
func (a *WireAdapter) EventsReply(instance string, sinceTick uint32, types []string) ([]byte, bool) {
	rep, ok := a.Manager.EventsReply(instance, sinceTick, types)
	if !ok {
		return nil, false
	}
	return frameReply(rep)
}

// ProbeReply frames the request_probe reply envelope for the legacy
// host:<instance> room (scraperiface.ProbeReply). (nil, false) when the
// manager could not service the probe or framing fails.
func (a *WireAdapter) ProbeReply(instance string) ([]byte, bool) {
	rep, ok := a.Manager.ProbeReply(instance)
	if !ok {
		return nil, false
	}
	return frameReply(rep)
}

// HelloPayloadFor is the manager's hello payload narrowed to what principal
// p may see (DESIGN-STEP6 §7.3 W-1, A.3): users, superusers and machine keys
// get every instance; a spectator / device key or the anonymous console door
// gets only the instance it is bound to, and only while that instance is
// live — authz.JoinableInstances is the filter. Classes and the protocol
// fields are not identity-dependent and stay as built.
func (a *WireAdapter) HelloPayloadFor(p authz.Principal) wire.HelloPayload {
	return a.Manager.HelloPayloadFiltered(func(names []string) []string {
		return authz.JoinableInstances(p, names)
	})
}

// SendHelloOn is a websocket.ConnectHook compatible signature. Plug it into
// websocket.NewHandler so every fresh client gets a hello envelope before
// any other scraper traffic flows:
//
//	se.Router.GET("/api/ws", ws.NewHandler(hub, app, adapter.SendHelloOn))
//
// Builds a fresh hello on each call so server_time and the instances list
// reflect the moment-of-connect state rather than a cached snapshot. The
// instances list is filtered to what p may join (HelloPayloadFor) so a
// bound key or console overlay never learns the other instances' names.
// Nothing is sent when the hello fails to marshal (logged).
func (a *WireAdapter) SendHelloOn(send func(data []byte), p authz.Principal) {
	if msgBytes, ok := helloMessageBytes(a.HelloPayloadFor(p)); ok {
		send(msgBytes)
	}
}

// helloMessageBytes marshals the hello envelope for payload and frames it as
// wire.Message{Type:"scraper"} with no room — exactly the bytes the manager
// used to enqueue itself. (nil, false) on marshal error (logged).
func helloMessageBytes(payload wire.HelloPayload) ([]byte, bool) {
	envBytes, ok := runner.HelloEnvelope(payload)
	if !ok {
		return nil, false
	}
	return wrapRoomMessage("hello", "", envBytes)
}

// replyRoom picks the WebSocket room for one runner.Reply per the table on
// WireAdapter. (room, false) when the class is not routable (logged).
func replyRoom(rep runner.Reply) (string, bool) {
	switch rep.Class {
	case wire.ClassEvents, wire.ClassProbe:
		// Legacy per-instance room: the request/reply channels predate
		// per-class rooms and clients still match replies on host:<inst>.
		// The name already passed this chokepoint at Manager.Start, so an
		// error here means a reply for an instance that never started.
		room, err := wire.RoomForInstance(rep.Instance)
		if err != nil {
			log.Printf("scraper[%s]: cannot resolve room for class %q: %v", rep.Instance, rep.Class, err)
			return "", false
		}
		return room, true
	default:
		return roomFor(rep.Instance, rep.Class)
	}
}

// frameReply wraps one reply as wire.Message bytes for its room.
func frameReply(rep runner.Reply) ([]byte, bool) {
	room, ok := replyRoom(rep)
	if !ok {
		return nil, false
	}
	return wrapRoomMessage(rep.Instance, room, rep.Envelope)
}

// frameReplies frames each reply in order, dropping any that fail. nil in →
// nil out, so the "runner does not exist" answer stays nil.
func frameReplies(reps []runner.Reply) [][]byte {
	if reps == nil {
		return nil
	}
	out := make([][]byte, 0, len(reps))
	for _, rep := range reps {
		if msg, ok := frameReply(rep); ok {
			out = append(out, msg)
		}
	}
	return out
}
