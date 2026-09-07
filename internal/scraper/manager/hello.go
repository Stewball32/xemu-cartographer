package manager

import (
	"encoding/json"
	"log"
	"time"

	"github.com/xemu-cartographer/xc-scraper/scraper"
	"github.com/xemu-cartographer/xc-scraper/wire"
)

// The hello envelope (envelopeTypeHello, classes.go) is the server→client
// handshake. Sent on WebSocket connect, before any other scraper traffic.
// Lets the client validate protocol compatibility and detect runner
// restarts by comparing per-instance started_at against any cached value.
// The connect hook itself (principal-filtered payload, wire.Message frame,
// send) lives in the league adapter — internal/leaguescraper
// WireAdapter.SendHelloOn — since step 7 part 3c.
//
// See atlas/new_json/04-ground-up-rebuild.md §6 (control), §7 (runner restart
// detection), §8 (versioning + handshake).

// HelloPayload is the data carried by a hello envelope. Alias of
// wire.HelloPayload.
type HelloPayload = wire.HelloPayload

// HelloInstance carries per-runner identity needed for runner-restart
// detection. StartedAt advances whenever a runner restarts (binary update,
// crash recovery, etc.); a reconnecting client compares it against the
// previously-seen value to detect that its cached per-class seq tracking is
// stale and to request fresh snapshots. Alias of wire.HelloInstance.
type HelloInstance = wire.HelloInstance

// BuildHelloPayload assembles the data for a hello envelope from the
// Manager's current view of the world. ServerTime is captured at call time
// so clients can estimate clock skew.
//
// Classes is the shared allClasses registry (classes.go) — every envelope
// class this server emits, the same list the sink reconciliation walks.
// Copied per call so a payload consumer can't mutate the registry.
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
		Classes:         append([]string(nil), allClasses...),
		Instances:       instances,
	}
}

// HelloPayloadFiltered is BuildHelloPayload narrowed to the instances keep
// retains. keep receives the full, name-sorted instance list and returns the
// subset a particular client may learn about (the league adapter passes
// authz.JoinableInstances for the connecting principal — DESIGN-STEP6 §7.3
// W-1, A.3: users, superusers and machine keys see everything; a spectator /
// device key or the anonymous console door only the instance it is bound
// to, and only while that instance is live). Names keep returns that are
// not in the list are ignored. Classes and the protocol fields are not
// identity-dependent and stay as built; Instances is never nil so it
// marshals as []. A nil keep is the unfiltered payload.
func (m *Manager) HelloPayloadFiltered(keep func(names []string) []string) HelloPayload {
	payload := m.BuildHelloPayload()
	if keep == nil {
		return payload
	}
	names := make([]string, 0, len(payload.Instances))
	for _, inst := range payload.Instances {
		names = append(names, inst.Name)
	}
	allowed := make(map[string]bool, len(names))
	for _, name := range keep(names) {
		allowed[name] = true
	}
	kept := make([]HelloInstance, 0, len(allowed))
	for _, inst := range payload.Instances {
		if allowed[inst.Name] {
			kept = append(kept, inst)
		}
	}
	payload.Instances = kept
	return payload
}

// HelloEnvelopeBytes builds the marshaled hello envelope — the
// scraper.Envelope wrapping the unfiltered HelloPayload — as bare envelope
// bytes. The league adapter frames it as a wire.Message (no room) and
// enqueues it on a single client's send channel; a per-principal hello goes
// through HelloPayloadFiltered + HelloEnvelope instead. Returns (nil, false)
// on marshal error (logged).
//
// The hello envelope's instance field is empty (hello is not per-instance)
// and its tick is 0. The payload's Instances list carries the per-instance
// metadata clients use for restart detection.
func (m *Manager) HelloEnvelopeBytes() ([]byte, bool) {
	return HelloEnvelope(m.BuildHelloPayload())
}

// HelloEnvelope marshals an already-built hello payload into the bare hello
// envelope bytes (type envelopeTypeHello, instance "", seq 0, tick 0).
// Returns (nil, false) on marshal error (logged).
func HelloEnvelope(payload HelloPayload) ([]byte, bool) {
	env := scraper.MakeEnvelope(envelopeTypeHello, "", 0, 0, payload)
	envBytes, err := json.Marshal(env)
	if err != nil {
		log.Printf("manager: marshal hello envelope: %v", err)
		return nil, false
	}
	return envBytes, true
}
