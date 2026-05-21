package handlers

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/Stewball32/xemu-cartographer/internal/websocket/rooms"
)

func init() {
	register("request_events", handleRequestEvents)
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
// stage 5d.
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
// Auth: free for any connected client; membership is the access gate
// (host:<name> rooms RequireAuth at join_room time).
func handleRequestEvents(e *Event) {
	if e.Services == nil || e.Services.Scraper == nil || e.Services.WS == nil {
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

	// Dedup by instance: a user in both host:<inst>:event and
	// host:<inst>:tick should only get one EventsReply per instance.
	seenInstance := map[string]bool{}
	for _, room := range e.Services.WS.UserRooms(e.UserID) {
		if room == rooms.HostAllRoom || room == rooms.SummaryRoom {
			continue
		}
		if !strings.HasPrefix(room, rooms.HostRoomPrefix+":") {
			continue
		}
		rest := strings.TrimPrefix(room, rooms.HostRoomPrefix+":")
		parts := strings.SplitN(rest, ":", 2)
		name := parts[0]
		if seenInstance[name] {
			continue
		}
		seenInstance[name] = true
		msgBytes, ok := e.Services.Scraper.EventsReply(name, filters.SinceTick, filters.Types)
		if !ok {
			continue
		}
		e.SendRaw(msgBytes)
	}
}
