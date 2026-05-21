package manager

import (
	"encoding/json"
	"log"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/websocket"
)

// envelopeTypeHello is the wire type for the server→client hello envelope.
// Sent on WebSocket connect, before any other scraper traffic. Lets the
// client validate protocol compatibility and detect runner restarts by
// comparing per-instance started_at against any cached value.
//
// See atlas/new_json/04-ground-up-rebuild.md §6 (control), §7 (runner restart
// detection), §8 (versioning + handshake). Emission lands in PR 3; this PR
// (PR 2 of 24) only ships the payload type + builder so the connect-handler
// in PR 3 has a self-contained data source to call.
const envelopeTypeHello = "hello"

// HelloPayload is the data carried by a hello envelope.
type HelloPayload struct {
	ProtocolVersion uint8           `json:"protocol_version"`
	ServerTime      time.Time       `json:"server_time"`
	Classes         []string        `json:"classes"`
	Instances       []HelloInstance `json:"instances"`
}

// HelloInstance carries per-runner identity needed for runner-restart
// detection. StartedAt advances whenever a runner restarts (binary update,
// crash recovery, etc.); a reconnecting client compares it against the
// previously-seen value to detect that its cached per-class seq tracking is
// stale and to request fresh snapshots.
type HelloInstance struct {
	Name      string    `json:"name"`
	StartedAt time.Time `json:"started_at"`
}

// BuildHelloPayload assembles the data for a hello envelope from the
// Manager's current view of the world. ServerTime is captured at call time
// so clients can estimate clock skew.
//
// Classes lists the envelope types this server emits. Today (v1 wire) that's
// the four existing envelope types; the v2 rollout broadens this to the
// per-data-class names (xbox, scenario, game, tick, ...) as they land.
func (m *Manager) BuildHelloPayload() HelloPayload {
	infos := m.List() // sorted by name
	instances := make([]HelloInstance, 0, len(infos))
	for _, info := range infos {
		instances = append(instances, HelloInstance{
			Name:      info.Name,
			StartedAt: info.StartedAt,
		})
	}
	return HelloPayload{
		ProtocolVersion: scraper.ProtocolVersion,
		ServerTime:      time.Now(),
		Classes: []string{
			envelopeTypeXbox,
			envelopeTypeScenario,
			envelopeTypeGame,
			envelopeTypeTick,
			envelopeTypeObjects,
			envelopeTypeDebug,
			envelopeTypeSummary,
			envelopeTypePreviousGame,
		},
		Instances: instances,
	}
}

// HelloEnvelopeBytes builds the marshaled wire bytes for a hello envelope —
// the websocket.Message wrapper plus the inner scraper.Envelope plus the
// HelloPayload — ready to enqueue on a single client's send channel.
// Returns (nil, false) on marshal error (logged); the caller is responsible
// for delivery.
//
// The hello envelope's instance field is empty (hello is not per-instance)
// and its tick is 0. The payload's Instances list carries the per-instance
// metadata clients use for restart detection.
func (m *Manager) HelloEnvelopeBytes() ([]byte, bool) {
	payload := m.BuildHelloPayload()
	env := scraper.MakeEnvelope(envelopeTypeHello, "", 0, 0, payload)
	envBytes, err := json.Marshal(env)
	if err != nil {
		log.Printf("manager: marshal hello envelope: %v", err)
		return nil, false
	}
	msg := websocket.Message{
		Type:    "scraper",
		Payload: envBytes,
	}
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		log.Printf("manager: marshal hello message: %v", err)
		return nil, false
	}
	return msgBytes, true
}

// SendHelloOn is a websocket.ConnectHook compatible signature. Plug it into
// websocket.NewHandler so every fresh client gets a hello envelope before
// any other scraper traffic flows.
//
//	se.Router.GET("/api/ws", ws.NewHandler(hub, app, scrMgr.SendHelloOn))
//
// Builds a fresh hello on each call so server_time and the instances list
// reflect the moment-of-connect state rather than a cached snapshot.
func (m *Manager) SendHelloOn(send func(data []byte)) {
	if msgBytes, ok := m.HelloEnvelopeBytes(); ok {
		send(msgBytes)
	}
}
