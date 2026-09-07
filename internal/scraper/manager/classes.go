package manager

import "github.com/xemu-cartographer/xc-scraper/wire"

// Envelope class names are owned by the wire contract package
// (xc-scraper/wire/classes.go). The envelopeType* constants below are
// re-declarations of the wire.Class* values under the manager's historical
// unexported names, so producers in this package keep reading naturally
// (envelopeTypeGame, envelopeTypeTick, ...) while the strings themselves have
// exactly one definition.
const (
	// envelopeTypeXbox — Xbox machine + XBE identity + kernel clock.
	envelopeTypeXbox = wire.ClassXbox
	// envelopeTypeScenario — the loaded map; one message per map load.
	envelopeTypeScenario = wire.ClassScenario
	// envelopeTypeGame — phase, freshness counters, config, roster, scores,
	// machines, network. Change-driven with a ≤1 Hz heartbeat floor.
	envelopeTypeGame = wire.ClassGame
	// envelopeTypeGameFiltered — the viewer-facing variant of game: identical
	// GamePayload shape, dummy roster removed server-side.
	envelopeTypeGameFiltered = wire.ClassGameFiltered
	// envelopeTypeTick — the hot path, ~30 Hz, volatile per-frame data only.
	envelopeTypeTick = wire.ClassTick
	// envelopeTypeObjects — world-object firehose + projectiles, opt-in.
	envelopeTypeObjects = wire.ClassObjects
	// envelopeTypeDebug — opt-in reverse-engineering class.
	envelopeTypeDebug = wire.ClassDebug
	// envelopeTypeSummary — cross-instance summary; broadcast to SummaryRoom.
	envelopeTypeSummary = wire.ClassSummary
	// envelopeTypePreviousGame — the just-finished game plus its complete
	// event log, once per game end and on subscribe.
	envelopeTypePreviousGame = wire.ClassPreviousGame
	// envelopeTypeEvent is the wire type for live per-event broadcasts. The
	// envelopes themselves are built inside the game plugins (see
	// xc-scraper/haloce/events — emit stamps Type:"event"); the manager only
	// routes them by Type.
	envelopeTypeEvent = wire.ClassEvent
	// envelopeTypeEventFiltered — viewer-facing deaths-only event stream.
	envelopeTypeEventFiltered = wire.ClassEventFiltered

	// envelopeTypeHello — the server→client handshake (hello.go).
	envelopeTypeHello = wire.ClassHello
	// envelopeTypeEvents — the request_events reply (events.go). Plural,
	// distinct from envelopeTypeEvent.
	envelopeTypeEvents = wire.ClassEvents
	// envelopeTypeProbe — the request_probe reply (probe.go).
	envelopeTypeProbe = wire.ClassProbe
)

// allClasses is the single source of truth for every envelope class this
// server emits — hello.go's handshake Classes list and sinks.go's
// applyPolicies reconciliation both consume it, so the two surfaces can't
// drift apart again. (They previously did: event, game_filtered and
// event_filtered were missing from both, which meant an event-class capture
// sink could never open.)
//
// DERIVED from wire.AllClasses (a fresh copy taken once at init); the
// per-class room table in internal/websocket/rooms/host.go is derived from
// the same registry (wire.PerInstanceClasses = allClasses minus summary), so
// the three surfaces are pinned by construction — see classes_test.go and
// rooms/scraper_classes_test.go.
var allClasses = wire.AllClasses()
