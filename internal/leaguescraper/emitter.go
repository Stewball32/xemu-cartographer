package leaguescraper

import (
	"encoding/json"
	"log"

	"github.com/Stewball32/xemu-cartographer/internal/guards"
	"github.com/xemu-cartographer/xc-scraper/runner"
	"github.com/xemu-cartographer/xc-scraper/wire"
)

// wsEmitter is the league server's runner.Emitter: it frames each envelope
// as a wire.Message{Type:"scraper"} and pushes it to the WebSocket room the
// pre-port manager used for that class.
//
// Room mapping (must stay byte-for-byte what the manager did before step 7
// part 3a — xc-scraper/runner runner.go emitClass/broadcastSnapshot/
// broadcastPoll, loop.go broadcast, aggregator.go broadcast):
//
//	instance == ""  (summary, the only cross-instance class)
//	    → wire.SummaryRoom                         ("host:summary")
//	instance != ""  (xbox, scenario, game, game_filtered, previous_game,
//	                 tick, objects, debug, event, event_filtered)
//	    → wire.RoomForInstanceClass(instance, class) ("host:<inst>:<class>")
//	      error → dropped (logged), exactly like the old per-class resolve
//
// No broadcast ever targeted the legacy per-instance room ("host:<inst>",
// wire.RoomForInstance): that room is only used by the request/reply
// channels (EventsReply, ProbeReply), which the WireAdapter (wireadapter.go)
// frames from the manager's bare runner.Reply values.
//
// svc.WS is read at call time, not at construction: main.go builds the
// manager before the hub exists and populates svc.WS later in OnServe, and
// the pre-port manager did the same late read. A nil hub drops the envelope.
type wsEmitter struct {
	svc *guards.Services
}

// NewEmitter returns the WebSocket-backed runner.Emitter for svc.
func NewEmitter(svc *guards.Services) runner.Emitter {
	return &wsEmitter{svc: svc}
}

func (e *wsEmitter) Emit(instance, class string, envBytes []byte) {
	if e == nil || e.svc == nil || e.svc.WS == nil {
		return
	}
	room, ok := roomFor(instance, class)
	if !ok {
		return
	}
	msgBytes, ok := wrapRoomMessage(instance, room, envBytes)
	if !ok {
		return
	}
	e.svc.WS.SendToRoomRaw(room, msgBytes)
}

// roomFor resolves the broadcast room for (instance, class) per the table
// on wsEmitter. (room, false) when the pair is unroutable.
func roomFor(instance, class string) (string, bool) {
	if instance == "" {
		return wire.SummaryRoom, true
	}
	room, err := wire.RoomForInstanceClass(instance, class)
	if err != nil {
		log.Printf("scraper[%s]: cannot resolve room for class %q: %v", instance, class, err)
		return "", false
	}
	return room, true
}

// wrapRoomMessage frames an already-marshalled wire.Envelope as the
// wire.Message{Type:"scraper", Room:room} the WS clients expect. Moved here
// from the manager (broadcast path in 3a, reply + hello paths in 3c);
// logged-and-dropped on marshal error. name is only for the log line.
func wrapRoomMessage(name, room string, envBytes []byte) ([]byte, bool) {
	msg := wire.Message{
		Type:    wire.TypeScraper,
		Room:    room,
		Payload: envBytes,
	}
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("scraper[%s]: marshal message: %v", name, err)
		return nil, false
	}
	return msgBytes, true
}
