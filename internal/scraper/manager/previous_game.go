package manager

import "github.com/xemu-cartographer/xc-scraper/wire"

// The previous_game class is sent once per game end (Live → Ready edge) and
// on subscribe: a snapshot of the just-finished game plus its complete
// event log. Its own class (rather than a field on game) because postgame
// views want it and overlays do not, and because it's a heavy payload that
// should never ride the heartbeat `game` stream.
//
// The payload shape and the end-reason set are owned by the wire contract
// package (xc-scraper/wire/previous_game.go); aliased / re-declared here.
//
// See atlas/new_json/04-ground-up-rebuild.md §2, §6 (`previous_game`).

// End reasons carried on previous_game (and projected into the games
// persistence chain, which stores them verbatim as an open set). Derived at
// the Live→Ready edge from the observed exit condition — see runLive.
const (
	// endReasonPostgame: the engine reached the postgame carousel — the
	// match ran to its natural end.
	endReasonPostgame = wire.EndReasonPostgame
	// endReasonLeftMatch: players left in_game without a postgame being
	// observed (quit to menu, or the lobby jumped straight to the next
	// pregame) — the artifact is a partial game, not a finished one.
	endReasonLeftMatch = wire.EndReasonLeftMatch
	// endReasonShutdown: the runner's context was cancelled mid-match
	// (daemon stop / instance teardown); the game was still in progress.
	endReasonShutdown = wire.EndReasonShutdown
)

// PreviousGamePayload is the data for a previous_game-class envelope.
// Events is the full per-game event log (oldest-first), in the same
// shape they appeared in live `event` envelopes; EventsTruncated flags
// the (pathological) matchEventsCap overflow where the log's tail was
// dropped. GameUID / EndReason are additive v2 fields — the stable
// per-game idempotency key and the observed exit condition.
type PreviousGamePayload = wire.PreviousGamePayload
