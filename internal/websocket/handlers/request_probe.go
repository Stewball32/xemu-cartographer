package handlers

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/Stewball32/xemu-cartographer/internal/websocket/rooms"
)

func init() {
	register("request_probe", handleRequestProbe)
}

// requestProbePayload mirrors the inbound WebSocket payload shape.
// `instance` lets a client target a specific runner without enumerating
// the rooms it's subscribed to (probe is a developer tool typically
// driven from the standalone probe page, which knows its instance by
// route param). When omitted, fall back to room-membership scanning
// like request_events does.
type requestProbePayload struct {
	Instance string `json:"instance,omitempty"`
}

// handleRequestProbe replies to the requester with the on-demand
// probe envelope for the named instance (or every host:<name> room
// the requester is in, if no instance was specified).
//
// Reply shape: one envelope of inner type "probe" per matched
// instance, addressed to room host:<name>. Payload is the
// ProbePayload (state_inputs + score_probe) freshly computed by the
// runner's loop goroutine — these never broadcast unsolicited.
//
// Auth: free for any connected client; membership is the access gate
// (host:<name> rooms RequireAuth at join_room time).
func handleRequestProbe(e *Event) {
	if e.Services == nil || e.Services.Scraper == nil || e.Services.WS == nil {
		return
	}

	var req requestProbePayload
	if len(e.Payload) > 0 {
		if err := json.Unmarshal(e.Payload, &req); err != nil {
			log.Printf("ws: request_probe: bad payload: %v", err)
		}
	}

	if req.Instance != "" {
		msgBytes, ok := e.Services.Scraper.ProbeReply(req.Instance)
		if ok {
			e.SendRaw(msgBytes)
		}
		return
	}

	// Fallback: scan room membership when no explicit instance was
	// supplied. Mirrors request_events' room-walk pattern, dedup'd by
	// instance name.
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
		msgBytes, ok := e.Services.Scraper.ProbeReply(name)
		if !ok {
			continue
		}
		e.SendRaw(msgBytes)
	}
}
