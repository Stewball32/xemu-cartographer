package events

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// detectGrenade emits player_update (kind=grenade_thrown) when a player's
// frag or plasma count drops while alive. Only fires for alive players —
// dropping grenades on death is not a throw.
func detectGrenade(ctx *Context) []scraper.Envelope {
	var out []scraper.Envelope

	for _, ip := range ctx.Result.InternalPlayers {
		idx := ip.Index
		tp := findTickPlayer(ctx.Result.Payload.Players, idx)
		if !tp.Alive {
			continue
		}
		player := playerRefByIndex(ctx.Snap, idx)
		pos := vec3FromTickPlayer(tp)

		if prevFrags := ctx.State.PrevFrags[idx]; tp.Frags < prevFrags {
			remaining := tp.Frags
			out = append(out, ctx.emitPlayerUpdate(scraper.PlayerUpdateEvent{
				Kind:        scraper.PlayerUpdateKindGrenadeThrown,
				Player:      player,
				Pos:         &pos,
				GrenadeType: scraper.GrenadeFrag,
				Remaining:   &remaining,
			}))
		}
		if prevPlasmas := ctx.State.PrevPlasmas[idx]; tp.Plasmas < prevPlasmas {
			remaining := tp.Plasmas
			out = append(out, ctx.emitPlayerUpdate(scraper.PlayerUpdateEvent{
				Kind:        scraper.PlayerUpdateKindGrenadeThrown,
				Player:      player,
				Pos:         &pos,
				GrenadeType: scraper.GrenadePlasma,
				Remaining:   &remaining,
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
