package scraper

// EventsReply builds the request_events response bytes for one instance.
// The response is a websocket.Message envelope (Type:"scraper") carrying a
// scraper.Envelope of type "events" addressed to room host:<name>; payload
// is the filtered, oldest-first event log (or an empty list when phase is
// not Live, per the M5 brief's resolution to OQ1).
//
// Filters:
//   - sinceTick: only events with envelope.Tick > sinceTick are included.
//   - types: only events whose envelope.Type is in the set; nil/empty means
//     no type filter.
//
// Returns (nil, false) if the named instance has no runner. Lives on the
// scraper service so handlers (which can't import internal/websocket
// directly without an import cycle) get pre-marshaled wire bytes back.
type EventsReply interface {
	EventsReply(instance string, sinceTick uint32, types []string) ([]byte, bool)
}