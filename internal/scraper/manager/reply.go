package manager

import (
	"encoding/json"
	"log"

	"github.com/xemu-cartographer/xc-scraper/scraper"
)

// Reply is one marshalled wire.Envelope handed back on a request/reply path
// (join replay, EventsReply, ProbeReply). The manager stops at the envelope:
// framing it as a wire.Message and choosing the WebSocket room is the league
// adapter's job (internal/leaguescraper WireAdapter), which keeps this
// package free of the server's transport.
//
// Instance + Class carry what the framer needs to pick the room:
//
//   - Class "summary" (Instance "") → the cross-instance summary room.
//   - Class "events" / "probe"      → the legacy per-instance room
//     ("host:<inst>", no class suffix) — the request/reply channels
//     predate per-class rooms and clients still match on it.
//   - every other class             → the per-class room
//     ("host:<inst>:<class>"), same as the broadcast path.
//
// Envelope is the bare marshalled envelope — identical bytes to what the
// Emitter port receives for the same class.
type Reply struct {
	Instance string
	Class    string
	Envelope []byte
}

// marshalReply serialises env into a Reply keyed by the envelope's own
// instance + type (the reply classes "events" / "probe" ride envelopes whose
// Type is the class). (Reply{}, false) on marshal error (logged).
func marshalReply(env scraper.Envelope) (Reply, bool) {
	envBytes, err := json.Marshal(env)
	if err != nil {
		log.Printf("scraper[%s]: marshal envelope (%s): %v", env.Instance, env.Type, err)
		return Reply{}, false
	}
	return Reply{Instance: env.Instance, Class: env.Type, Envelope: envBytes}, true
}
