package leaguescraper

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/xcclient"
	"github.com/xemu-cartographer/xc-scraper/runner"
	"github.com/xemu-cartographer/xc-scraper/wire"
)

// upstreamHello remembers the protocol fields of the last hello the daemon
// sent (DESIGN-STEP8 §8.5). Until the first upstream hello arrives it
// answers this build's wire.ProtocolVersion + wire.AllClasses(), so a
// downstream client connecting before the daemon is up never sees
// classes: [] — the class set is a build-time constant of the contract,
// not something the daemon is allowed to shrink at runtime.
type upstreamHello struct {
	mu      sync.RWMutex
	version uint8
	classes []string
	seen    bool
}

func (h *upstreamHello) reset() {
	h.mu.Lock()
	h.version = wire.ProtocolVersion
	h.classes = wire.AllClasses()
	h.seen = false
	h.mu.Unlock()
}

// observe records the protocol fields when f is an upstream hello.
func (h *upstreamHello) observe(f xcclient.Frame) {
	if f.Env == nil || f.Env.Type != wire.ClassHello {
		return
	}
	var hp wire.HelloPayload
	if err := json.Unmarshal(f.Env.Data, &hp); err != nil {
		log.Printf("leaguescraper: undecodable upstream hello: %v", err)
		return
	}
	h.mu.Lock()
	h.version = hp.ProtocolVersion
	if len(hp.Classes) != 0 {
		h.classes = append([]string(nil), hp.Classes...)
	}
	h.seen = true
	h.mu.Unlock()
}

// fields returns the current (version, classes copy, seen).
func (h *upstreamHello) fields() (uint8, []string, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.version, append([]string(nil), h.classes...), h.seen
}

// UpstreamHelloSeen reports whether a daemon hello has been observed since
// the adapter was built.
func (a *Adapter) UpstreamHelloSeen() bool {
	_, _, seen := a.hello.fields()
	return seen
}

// HelloPayloadFor builds the hello handshake for p: protocol_version +
// classes from the last upstream hello (defaults above), server_time now,
// and the instances p may join (authz.JoinableInstances over the mirror's
// instance set, with each instance's mirrored started_at) — a spectator /
// device key or the anonymous console door learns only the instance it is
// bound to, and only while that instance is live. Always a non-nil
// instances slice so the JSON is [] while empty.
func (a *Adapter) HelloPayloadFor(p authz.Principal) wire.HelloPayload {
	version, classes, _ := a.hello.fields()
	names := a.mirror.Instances()
	joinable := authz.JoinableInstances(p, names)
	instances := make([]wire.HelloInstance, 0, len(joinable))
	for _, name := range joinable {
		startedAt, _ := a.mirror.StartedAt(name)
		instances = append(instances, wire.HelloInstance{Name: name, StartedAt: startedAt})
	}
	return wire.HelloPayload{
		ProtocolVersion: version,
		ServerTime:      time.Now(),
		Classes:         classes,
		Instances:       instances,
	}
}

// SendHelloOn is the websocket.ConnectHook the /api/ws handler takes
// (ws.NewHandler(hub, app, adapter.SendHelloOn)): every fresh client gets
// its hello before any other scraper traffic. Framed exactly like the
// in-process adapter's (wire.Message{Type:"scraper"}, no room, hello
// envelope); nothing is sent when the hello fails to marshal (logged).
func (a *Adapter) SendHelloOn(send func(data []byte), p authz.Principal) {
	if msgBytes, ok := helloFrame(a.HelloPayloadFor(p)); ok {
		send(msgBytes)
	}
}

// helloFrame marshals the hello envelope for payload and frames it as
// wire.Message{Type:"scraper"} with no room.
func helloFrame(payload wire.HelloPayload) ([]byte, bool) {
	envBytes, ok := runner.HelloEnvelope(payload)
	if !ok {
		return nil, false
	}
	msgBytes, err := json.Marshal(wire.Message{Type: wire.TypeScraper, Payload: envBytes})
	if err != nil {
		log.Printf("leaguescraper: marshal hello message: %v", err)
		return nil, false
	}
	return msgBytes, true
}
