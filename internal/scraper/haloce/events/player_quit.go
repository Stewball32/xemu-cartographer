package events

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// detectPlayerQuit emits player_quit on the QuitFlag 0 → 1 transition. Note
// this is the engine's quit flag, distinct from a player-disappeared-from-
// roster event — see roster.go for the latter.
func detectPlayerQuit(ctx *Context) []scraper.Envelope {
	var out []scraper.Envelope
	for _, ip := range ctx.Result.InternalPlayers {
		idx := ip.Index
		prev, ok := ctx.State.PrevQuit[idx]
		if !ok {
			continue
		}
		if prev == 0 && ip.QuitFlag == 1 {
			out = append(out, ctx.emit(map[string]any{
				"event_type": scraper.EventPlayerQuit,
				"player":     idx,
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

// updateWeaponSlotsPrev preserves the legacy bookkeeping of PrevWeaponSlots.
// No detector currently consumes it but UpdateTickState in the original
// events.go wrote it, so keep parity until a downstream consumer is removed.
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
