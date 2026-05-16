package manager

import (
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
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
			envelopeTypeCurrentState,
			envelopeTypeStateUpdate,
			envelopeTypeEvent,
			envelopeTypeEvents,
		},
		Instances: instances,
	}
}
