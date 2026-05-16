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

// PreviousGamePayload is the data for a previous_game-class envelope.
// Events is the full per-game event log (oldest-first), in the same
// shape they appeared in live `event` envelopes.
type PreviousGamePayload struct {
	EndedAt time.Time          `json:"ended_at"`
	Game    *GamePayload       `json:"game"`
	Events  []scraper.Envelope `json:"events"`
}
