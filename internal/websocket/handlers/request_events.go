package handlers

import (
	"encoding/json"
	"log"

	"github.com/xemu-cartographer/xemu-cartographer/internal/authz"
)

func init() {
	register("request_events", async(handleRequestEvents))
}

// async wraps a request handler so it runs off the Hub's dispatch goroutine
// (DESIGN-STEP8 §8.5, F2). request_events / request_probe end in a scraper
// round-trip that in wire mode is an HTTP hop to the daemon; run inline it
// would stall the Run loop — and every other client's frames — for the
// duration of that upstream I/O. Everything the handler touches is
// goroutine-safe: the principal is a value copy, the authz deps and the
// scraper adapter serve HTTP handlers concurrently already, Rooms reads
// under the Hub's RLock and SendRaw is trySend, which after the client's
// removal drops the frame rather than sending on a closed channel (send is
// never closed). The dispatch-time decisions (registered sender, kind
// whitelist) already ran on the Run goroutine before the handler was
// looked up.
func async(fn HandlerFunc) HandlerFunc {
	return func(e *Event) { go fn(e) }
}

// requestEventsPayload mirrors the inbound WebSocket payload shape:
// optional filters for tick + types. With neither, the full Live-phase
// event log (newest 50 entries, capped by recentEventsCap on the runner)
// is returned. Unknown fields are ignored.
type requestEventsPayload struct {
	SinceTick uint32   `json:"since_tick,omitempty"`
	Types     []string `json:"types,omitempty"`
}

// handleRequestEvents replies to the requester with the recent event log
// for each host:<name> room they are subscribed to, filtered by the
// optional since_tick + types parameters in the request payload. M5
// stage 5d. Registered through async: the Hub dispatches it on its own
// goroutine, so a slow Replayer never blocks the Run loop.
//
// Reply shape: one envelope per host:<name> room the requester is in,
// of inner type "events" (plural; distinct from per-event live "event"
// envelopes). Payload carries phase, the echoed since_tick, and the
// filtered events in oldest-first order. host:all is skipped — there is
// no per-aggregate event log.
//
// In Idle and Ready phases, Events is always empty even when previous_game
// exists (M5 brief OQ1 resolution). Phase comes back so a client knows
// whether it received an empty list because the runner has no current-
// match log or because no events match its filters.
//
// Auth: membership selects the instances; authz.Can(scraper.events,
// Instance(name)) then decides each one (scope, roster / box ownership for
// users, the binding for spectator / device keys). Denied instances are
// skipped silently.
func handleRequestEvents(e *Event) {
	if e.Services == nil || e.Services.Scraper == nil || e.Rooms == nil {
		return
	}

	var filters requestEventsPayload
	if len(e.Payload) > 0 {
		if err := json.Unmarshal(e.Payload, &filters); err != nil {
			// Malformed payloads degrade to "no filters". Logged so a
			// client breakage during 5e rollout is visible server-side
			// without a user-facing failure.
			log.Printf("ws: request_events: bad payload: %v", err)
		}
	}

	// Dedup by instance: a sender in both host:<inst>:event and
	// host:<inst>:tick should only get one EventsReply per instance.
	seenInstance := map[string]bool{}
	for _, room := range e.Rooms() {
		name, ok := instanceOfRoom(room)
		if !ok || seenInstance[name] {
			continue
		}
		seenInstance[name] = true
		if !authz.Can(e.Authz, e.Principal, authz.ActionScraperEvents, authz.Instance(name)) {
			continue
		}
		msgBytes, ok := e.Services.Scraper.EventsReply(name, filters.SinceTick, filters.Types)
		if !ok {
			continue
		}
		e.SendRaw(msgBytes)
	}
}
