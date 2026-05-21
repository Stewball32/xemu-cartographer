package events

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// detectPowerup emits player_update events for active camo and overshield
// state transitions: kind=powerup_picked_up on the false → true edge,
// kind=powerup_expired on the true → false edge. Expiry only fires when
// the player is still alive (avoids double-firing alongside death).
func detectPowerup(ctx *Context) []scraper.Envelope {
	var out []scraper.Envelope

	for _, ip := range ctx.Result.InternalPlayers {
		idx := ip.Index
		tp := findTickPlayer(ctx.Result.Payload.Players, idx)
		pos := vec3FromTickPlayer(tp)
		player := playerRefByIndex(ctx.Snap, idx)

		// --- active camouflage ---
		prevCamo := ctx.State.PrevHasCamo[idx]
		if !prevCamo && tp.HasCamo {
			out = append(out, ctx.emitPlayerUpdate(scraper.PlayerUpdateEvent{
				Kind:    scraper.PlayerUpdateKindPowerupPickedUp,
				Player:  player,
				Pos:     &pos,
				Powerup: scraper.PowerupActiveCamouflage,
			}))
		}
		if prevCamo && !tp.HasCamo && tp.Alive {
			out = append(out, ctx.emitPlayerUpdate(scraper.PlayerUpdateEvent{
				Kind:    scraper.PlayerUpdateKindPowerupExpired,
				Player:  player,
				Pos:     &pos,
				Powerup: scraper.PowerupActiveCamouflage,
			}))
		}

		// --- overshield ---
		prevOS := ctx.State.PrevHasOvershield[idx]
		if !prevOS && tp.HasOvershield {
			out = append(out, ctx.emitPlayerUpdate(scraper.PlayerUpdateEvent{
				Kind:    scraper.PlayerUpdateKindPowerupPickedUp,
				Player:  player,
				Pos:     &pos,
				Powerup: scraper.PowerupOvershield,
			}))
		}
		if prevOS && !tp.HasOvershield && tp.Alive {
			out = append(out, ctx.emitPlayerUpdate(scraper.PlayerUpdateEvent{
				Kind:    scraper.PlayerUpdateKindPowerupExpired,
				Player:  player,
				Pos:     &pos,
				Powerup: scraper.PowerupOvershield,
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
