package events

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// detectPowerup emits powerup_picked_up / powerup_expired for active camo
// and overshield. Pickup fires unconditionally on the false → true edge;
// expiry only fires if the player is still alive (avoids double-firing
// expiry alongside death).
func detectPowerup(ctx *Context) []scraper.Envelope {
	var out []scraper.Envelope

	for _, ip := range ctx.Result.InternalPlayers {
		idx := ip.Index
		tp := findTickPlayer(ctx.Result.Payload.Players, idx)

		// --- active camouflage ---
		prevCamo := ctx.State.PrevHasCamo[idx]
		if !prevCamo && tp.HasCamo {
			out = append(out, ctx.emit(map[string]any{
				"event_type": scraper.EventPowerupPickup,
				"player":     idx,
				"kind":       "active_camouflage",
			}))
		}
		if prevCamo && !tp.HasCamo && tp.Alive {
			out = append(out, ctx.emit(map[string]any{
				"event_type": scraper.EventPowerupExpired,
				"player":     idx,
				"kind":       "active_camouflage",
			}))
		}

		// --- overshield ---
		prevOS := ctx.State.PrevHasOvershield[idx]
		if !prevOS && tp.HasOvershield {
			out = append(out, ctx.emit(map[string]any{
				"event_type": scraper.EventPowerupPickup,
				"player":     idx,
				"kind":       "overshield",
			}))
		}
		if prevOS && !tp.HasOvershield && tp.Alive {
			out = append(out, ctx.emit(map[string]any{
				"event_type": scraper.EventPowerupExpired,
				"player":     idx,
				"kind":       "overshield",
			}))
		}
	}
	return out
}

func updatePowerupPrev(state *scraper.TickState, result scraper.TickResult) {
	for _, tp := range result.Payload.Players {
		state.PrevHasCamo[tp.Index] = tp.HasCamo
		state.PrevHasOvershield[tp.Index] = tp.HasOvershield
	}
}

func init() {
	RegisterDetector(detectPowerup)
	RegisterUpdater(updatePowerupPrev)
}
