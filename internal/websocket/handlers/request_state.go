package handlers

func init() {
	register("request_state", handleRequestState)
}

// handleRequestState replies to the requesting client with the most recent
// cached snapshot for every running scraper. Useful for clients that want to
// re-sync state without leaving and re-joining the overlay room — the same
// data is sent to mid-match joiners via join_room, but request_state lets a
// long-lived client refresh on demand (e.g. after a network blip).
//
// Auth: free for any connected client. The cached snapshots themselves are
// only built for runners that exist, and the scraper service is only present
// when CONTAINERS_ENABLED=true, so an unauthenticated client connecting in
// a development environment with no scrapers gets an empty response.
func handleRequestState(e *Event) {
	if e.Services == nil || e.Services.Scraper == nil {
		return
	}
	for _, snap := range e.Services.Scraper.LatestSnapshotMessages() {
		e.SendRaw(snap)
	}
}
