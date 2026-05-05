package handlers

func init() {
	register("request_state", handleRequestState)
}

// handleRequestState replies to the requesting client with the most recent
// cached game data for every running scraper. Useful for clients that want
// to re-sync state without leaving and re-joining their host room — the
// same bytes are sent to mid-match joiners via join_room, but request_state
// lets a long-lived client refresh on demand (e.g. after a network blip).
//
// Auth: free for any connected client. The cached bytes are only built for
// runners that have observed game data; the scraper service is only
// present when CONTAINERS_ENABLED=true, so an unauthenticated client
// connecting in a development environment with no scrapers gets an empty
// response.
//
// TODO(M5 stage 5d): narrow this to a single-room reply built from the new
// current_state envelope shape. Today it returns one envelope per runner
// regardless of which host:<name> room the requester is in; 5d will look
// up the requester's room membership and reply only for matching rooms.
func handleRequestState(e *Event) {
	if e.Services == nil || e.Services.Scraper == nil {
		return
	}
	for _, msg := range e.Services.Scraper.JoinReplayMessages() {
		e.SendRaw(msg)
	}
}
