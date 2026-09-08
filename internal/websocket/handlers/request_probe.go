package handlers

import (
	"encoding/json"
	"log"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
)

func init() {
	register("request_probe", async(handleRequestProbe))
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
// the requester is in, if no instance was specified). Registered through
// async (request_events.go): the probe round-trip runs off the Hub's Run
// goroutine.
//
// Reply shape: one envelope of inner type "probe" per matched
// instance, addressed to room host:<name>. Payload is the
// ProbePayload (state_inputs + score_probe) freshly computed by the
// runner's loop goroutine — these never broadcast unsolicited.
//
// Auth: authz.Can(scraper.probe, Instance(name)) per instance — a
// scraper.* scope (admin / machine keys); anything else is silently
// dropped, the same shape as an unknown instance. Room membership alone
// does not grant it: the probe exposes raw memory reads.
func handleRequestProbe(e *Event) {
	if e.Services == nil || e.Services.Scraper == nil {
		return
	}

	var req requestProbePayload
	if len(e.Payload) > 0 {
		if err := json.Unmarshal(e.Payload, &req); err != nil {
			log.Printf("ws: request_probe: bad payload: %v", err)
		}
	}

	if req.Instance != "" {
		if !authz.Can(e.Authz, e.Principal, authz.ActionScraperProbe, authz.Instance(req.Instance)) {
			return
		}
		msgBytes, ok := e.Services.Scraper.ProbeReply(req.Instance)
		if ok {
			e.SendRaw(msgBytes)
		}
		return
	}

	// Fallback: scan the sender's own room memberships when no explicit
	// instance was supplied. Mirrors request_events' room-walk pattern,
	// dedup'd by instance name, with the same per-instance check.
	seenInstance := map[string]bool{}
	if e.Rooms == nil {
		return
	}
	for _, room := range e.Rooms() {
		name, ok := instanceOfRoom(room)
		if !ok || seenInstance[name] {
			continue
		}
		seenInstance[name] = true
		if !authz.Can(e.Authz, e.Principal, authz.ActionScraperProbe, authz.Instance(name)) {
			continue
		}
		msgBytes, ok := e.Services.Scraper.ProbeReply(name)
		if !ok {
			continue
		}
		e.SendRaw(msgBytes)
	}
}
