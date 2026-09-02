package manager

import (
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// envelopeTypePreviousGame is the wire type for the per-instance
// previous_game class — sent once per game end (Live → Ready edge) and
// on subscribe. Snapshot of the just-finished game plus its complete
// event log.
//
// Its own class (rather than a field on game) because postgame views
// want it and overlays do not, and because it's a heavy payload that
// should never ride the heartbeat `game` stream.
//
// See atlas/new_json/04-ground-up-rebuild.md §2, §6 (`previous_game`).
const envelopeTypePreviousGame = "previous_game"

// End reasons carried on previous_game (and projected into the games
// persistence chain, which stores them verbatim as an open set). Derived at
// the Live→Ready edge from the observed exit condition — see runLive.
const (
	// endReasonPostgame: the engine reached the postgame carousel — the
	// match ran to its natural end.
	endReasonPostgame = "postgame"
	// endReasonLeftMatch: players left in_game without a postgame being
	// observed (quit to menu, or the lobby jumped straight to the next
	// pregame) — the artifact is a partial game, not a finished one.
	endReasonLeftMatch = "left_match"
	// endReasonShutdown: the runner's context was cancelled mid-match
	// (daemon stop / instance teardown); the game was still in progress.
	endReasonShutdown = "shutdown"
)

// PreviousGamePayload is the data for a previous_game-class envelope.
// Events is the full per-game event log (oldest-first), in the same
// shape they appeared in live `event` envelopes; EventsTruncated flags
// the (pathological) matchEventsCap overflow where the log's tail was
// dropped. GameUID / EndReason are additive v2 fields — the stable
// per-game idempotency key and the observed exit condition.
type PreviousGamePayload struct {
	EndedAt         time.Time          `json:"ended_at"`
	Game            *GamePayload       `json:"game"`
	Events          []scraper.Envelope `json:"events"`
	EventsTruncated bool               `json:"events_truncated"`
	GameUID         string             `json:"game_uid"`
	EndReason       string             `json:"end_reason"`
}
