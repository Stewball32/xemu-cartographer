package events

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// detectMatch emits game_update events on the game-state transitions into
// in_game (kind=game_start) and postgame (kind=game_end). The
// PrevGameState field on TickState is the source of truth;
// finalizeMatchPrev advances it at the end of each Detect dispatch.
func detectMatch(ctx *Context) []scraper.Envelope {
	var out []scraper.Envelope
	prev := ctx.State.PrevGameState
	cur := ctx.Snap.GameState

	if prev == cur {
		return out
	}

	switch cur {
	case scraper.GameStateInGame:
		out = append(out, ctx.emitGameUpdate(scraper.GameUpdateEvent{
			Kind:     scraper.GameUpdateKindGameStart,
			Map:      ctx.Snap.Map,
			Gametype: ctx.Snap.Gametype,
		}))
	case scraper.GameStatePostGame:
		// Only emit game_end on the in_game → postgame edge — entering
		// postgame from anywhere else (menu, pregame) isn't a game
		// completion in the user-facing sense.
		if prev == scraper.GameStateInGame {
			out = append(out, ctx.emitGameUpdate(scraper.GameUpdateEvent{
				Kind:     scraper.GameUpdateKindGameEnd,
				Map:      ctx.Snap.Map,
				Gametype: ctx.Snap.Gametype,
			}))
		}
	}
	return out
}

func updateMatchPrev(state *scraper.TickState, result scraper.TickResult) {
	// Bookkeeping happens inline at the end of detectMatch via
	// finalizeMatchPrev, since the diff source is game data (not result).
	_ = state
	_ = result
}

func finalizeMatchPrev(state *scraper.TickState, snap scraper.GameData) {
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
