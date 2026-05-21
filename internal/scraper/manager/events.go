package manager

import (
	"encoding/json"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// envelopeTypeEvents is the wire type for a request_events response.
// Plural, distinct from envelopeTypeEvent — clients pattern-match on the
// inner envelope type to tell a request reply ("events") from a live
// per-event broadcast ("event"). One reply envelope per instance.
const envelopeTypeEvents = "events"

// EventsResponsePayload is the payload of a request_events reply envelope.
// Phase is included so a client receiving an empty Events list knows
// whether the runner is in Live (no matching events) or Idle/Ready (always
// empty per the M5 brief's resolution to OQ1). SinceTick echoes back the
// requester's filter so they can match the response to the request.
type EventsResponsePayload struct {
	Phase     Phase              `json:"phase"`
	SinceTick uint32             `json:"since_tick"`
	Events    []scraper.Envelope `json:"events"`
}

// EventsReply builds the request_events response bytes for one instance.
// See scraperiface.EventsReply for the wire contract. The cache stores
// events newest-first; this method reverses on the way out so clients
// get oldest-first stream order (M5 brief OQ7 resolution).
func (m *Manager) EventsReply(instance string, sinceTick uint32, types []string) ([]byte, bool) {
	m.mu.Lock()
	r, ok := m.runners[instance]
	m.mu.Unlock()
	if !ok {
		return nil, false
	}

	c := r.readCache()

	var events []scraper.Envelope
	if c.Phase == PhaseLive {
		events = filterEvents(c.Events, sinceTick, types)
	}
	if events == nil {
		events = []scraper.Envelope{}
	}

	payload := EventsResponsePayload{
		Phase:     c.Phase,
		SinceTick: sinceTick,
		Events:    events,
	}
	env := scraper.MakeEnvelope(envelopeTypeEvents, instance, 0, c.EngineTick, payload)

	msgBytes, ok := marshalRoomMessage(instance, r.hostRoom, env)
	if !ok {
		return nil, false
	}
	return msgBytes, true
}

// filterEvents returns events with tick > sinceTick AND semantic event-type
// in types (if types is non-nil), in oldest-first order. The input is the
// cache's newest-first event log (see runner.pushEvent); this iterates
// back-to-front to flip ordering.
//
// Type matching uses the inner payload's event_type field. The outer
// envelope.Type is always the literal "event" wire-type for every event,
// so matching against it would only ever pass for types=["event"].
func filterEvents(events []scraper.Envelope, sinceTick uint32, types []string) []scraper.Envelope {
	if len(events) == 0 {
		return []scraper.Envelope{}
	}
	var typeSet map[string]struct{}
	if len(types) > 0 {
		typeSet = make(map[string]struct{}, len(types))
		for _, t := range types {
			typeSet[t] = struct{}{}
		}
	}
	out := make([]scraper.Envelope, 0, len(events))
	for i := len(events) - 1; i >= 0; i-- {
		ev := events[i]
		if ev.Tick <= sinceTick {
			continue
		}
		if typeSet != nil {
			if _, ok := typeSet[eventInnerType(ev)]; !ok {
				continue
			}
		}
		out = append(out, ev)
	}
	return out
}

// eventInnerType extracts the semantic event type from the envelope's
// payload (e.g. "kill", "spawn", "team_score"). Returns "" if the payload
// is missing or the field can't be decoded — those events fail any
// type-set filter rather than spuriously matching.
func eventInnerType(ev scraper.Envelope) string {
	if len(ev.Data) == 0 {
		return ""
	}
	var holder struct {
		EventType string `json:"event_type"`
	}
	if err := json.Unmarshal(ev.Data, &holder); err != nil {
		return ""
	}
	return holder.EventType
}
