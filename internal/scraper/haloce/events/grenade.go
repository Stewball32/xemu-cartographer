package events

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// detectGrenade emits grenade_thrown when a player's frag or plasma count
// drops while alive.
func detectGrenade(ctx *Context) []scraper.Envelope {
	var out []scraper.Envelope

	for _, ip := range ctx.Result.InternalPlayers {
		idx := ip.Index
		tp := findTickPlayer(ctx.Result.Payload.Players, idx)
		if !tp.Alive {
			continue
		}

		if prevFrags := ctx.State.PrevFrags[idx]; tp.Frags < prevFrags {
			out = append(out, ctx.emit(map[string]any{
				"event_type":      scraper.EventGrenadeThrown,
				"player":          idx,
				"kind":            "frag",
				"frags_remaining": tp.Frags,
			}))
		}
		if prevPlasmas := ctx.State.PrevPlasmas[idx]; tp.Plasmas < prevPlasmas {
			out = append(out, ctx.emit(map[string]any{
				"event_type":        scraper.EventGrenadeThrown,
				"player":            idx,
				"kind":              "plasma",
				"plasmas_remaining": tp.Plasmas,
			}))
		}
	}
	return out
}

func updateGrenadePrev(state *scraper.TickState, result scraper.TickResult) {
	for _, tp := range result.Payload.Players {
		state.PrevFrags[tp.Index] = tp.Frags
		state.PrevPlasmas[tp.Index] = tp.Plasmas
	}
}

func init() {
	RegisterDetector(detectGrenade)
	RegisterUpdater(updateGrenadePrev)
}
