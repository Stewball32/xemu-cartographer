package events

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// detectPlayerQuit emits player_update (kind=player_quit) on the engine
// quit-flag 0 → 1 transition. Distinct from a player-disappeared-from-
// roster event (see roster.go's player_left); the engine quit flag means
// "this player pressed quit" rather than "their slot has been recycled."
func detectPlayerQuit(ctx *Context) []scraper.Envelope {
	var out []scraper.Envelope
	for _, ip := range ctx.Result.InternalPlayers {
		idx := ip.Index
		prev, ok := ctx.State.PrevQuit[idx]
		if !ok {
			continue
		}
		if prev == 0 && ip.QuitFlag == 1 {
			out = append(out, ctx.emitPlayerUpdate(scraper.PlayerUpdateEvent{
				Kind:   scraper.PlayerUpdateKindPlayerQuit,
				Player: playerRefByIndex(ctx.Snap, idx),
			}))
		}
	}
	return out
}

func updatePlayerQuitPrev(state *scraper.TickState, result scraper.TickResult) {
	for _, ip := range result.InternalPlayers {
		state.PrevQuit[ip.Index] = ip.QuitFlag
	}
}

// updateWeaponSlotsPrev preserves legacy bookkeeping of PrevWeaponSlots.
// No detector currently consumes it but UpdateTickState in the original
// events.go wrote it, so keep parity until a downstream consumer is
// confirmed removed.
func updateWeaponSlotsPrev(state *scraper.TickState, result scraper.TickResult) {
	for _, ip := range result.InternalPlayers {
		state.PrevWeaponSlots[ip.Index] = ip.WeaponSlots
	}
}

func init() {
	RegisterDetector(detectPlayerQuit)
	RegisterUpdater(updatePlayerQuitPrev)
	RegisterUpdater(updateWeaponSlotsPrev)
}
