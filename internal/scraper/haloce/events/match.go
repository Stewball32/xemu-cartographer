package events

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// detectMatch emits game_start and game_end on the snapshot's GameState
// transitioning into in_game / postgame respectively. Today these
// transitions are observed by the manager loop — but emitting them through
// the events pipeline keeps the wire shape uniform (every match event flows
// through Detect → events broadcast).
//
// The PrevGameState field on TickState is the source of truth; the loop
// no longer needs to emit these directly. Final state-update happens in
// updateMatchPrev so the next tick can diff.
func detectMatch(ctx *Context) []scraper.Envelope {
	var out []scraper.Envelope
	prev := ctx.State.PrevGameState
	cur := ctx.Snap.GameState

	if prev != cur {
		switch cur {
		case scraper.GameStateInGame:
			out = append(out, ctx.emit(map[string]any{
				"event_type": scraper.EventGameStart,
				"map":        ctx.Snap.Map,
				"gametype":   ctx.Snap.Gametype,
			}))
		case scraper.GameStatePostGame:
			// Only emit game_end on the in_game → postgame edge — entering
			// postgame from anywhere else (menu, pregame) isn't a match
			// completion in the user-facing sense.
			if prev == scraper.GameStateInGame {
				out = append(out, ctx.emit(map[string]any{
					"event_type": scraper.EventGameEnd,
					"map":        ctx.Snap.Map,
					"gametype":   ctx.Snap.Gametype,
				}))
			}
		}
	}
	return out
}

func updateMatchPrev(state *scraper.TickState, result scraper.TickResult) {
	// Match-state diff source is the snapshot, not the tick result. Like
	// roster.go, this updater is a no-op and the bookkeeping happens
	// inline at the end of detectMatch via finalizeMatchPrev.
	_ = state
	_ = result
}

func finalizeMatchPrev(state *scraper.TickState, snap scraper.SnapshotPayload) {
	state.PrevGameState = snap.GameState
}

func init() {
	RegisterDetector(func(ctx *Context) []scraper.Envelope {
		out := detectMatch(ctx)
		finalizeMatchPrev(ctx.State, ctx.Snap)
		return out
	})
	RegisterUpdater(updateMatchPrev) // no-op; bookkeeping happens inline
}
