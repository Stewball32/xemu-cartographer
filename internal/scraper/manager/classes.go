package manager

// envelopeTypeEvent is the wire type for live per-event broadcasts. The
// envelopes themselves are built inside the game plugins (see
// internal/scraper/haloce/events — emit stamps Type:"event"); the manager
// only routes them by Type, so the constant lives here beside the class
// registry rather than in a payload file of its own.
const envelopeTypeEvent = "event"

// allClasses is the single source of truth for every envelope class this
// server emits — hello.go's handshake Classes list and sinks.go's
// applyPolicies reconciliation both consume it, so the two surfaces can't
// drift apart again. (They previously did: event, game_filtered and
// event_filtered were missing from both, which meant an event-class capture
// sink could never open.) Keep in sync with the per-class room table in
// internal/websocket/rooms/host.go — every entry except summary (which has
// its own cross-instance SummaryRoom) must be a registered scraper class.
var allClasses = []string{
	envelopeTypeXbox,
	envelopeTypeScenario,
	envelopeTypeGame,
	envelopeTypeGameFiltered,
	envelopeTypeTick,
	envelopeTypeObjects,
	envelopeTypeDebug,
	envelopeTypeSummary,
	envelopeTypePreviousGame,
	envelopeTypeEvent,
	envelopeTypeEventFiltered,
}
