package scraper

// ProbeReply builds the request_probe response bytes for one instance.
// The response is a websocket.Message envelope (Type:"scraper") carrying
// a scraper.Envelope of type "probe" addressed to room host:<name>;
// payload is ProbePayload{state_inputs, score_probe}.
//
// state_inputs is the plugin's most recent ReadGameState sample (cheap
// pointer-return from the reader's internal cache). score_probe is the
// free-form bag returned by the plugin's BuildScoreProbe — expensive
// (multiple memory-region samples) and the whole reason this lives on
// its own request/reply channel rather than the per-tick debug envelope.
//
// Returns (nil, false) when the named instance has no runner, the
// runner is in Idle (no reader bound; reply carries empty maps), or
// the loop didn't service the request within probeReplyTimeout. Lives
// on the scraper service so handlers get pre-marshaled wire bytes back
// without an import cycle into internal/websocket.
type ProbeReply interface {
	ProbeReply(instance string) ([]byte, bool)
}
