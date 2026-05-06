package events

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// detectDamage emits damage and melee events. Both share the damage-table
// helpers, so they live in one file.
func detectDamage(ctx *Context) []scraper.Envelope {
	var out []scraper.Envelope

	for _, ip := range ctx.Result.InternalPlayers {
		idx := ip.Index
		tp := findTickPlayer(ctx.Result.Payload.Players, idx)
		prevAlive := ctx.State.PrevAlive[idx]

		// --- damage ---
		prevHP := ctx.State.PrevHealth[idx] + ctx.State.PrevShields[idx]
		currHP := tp.Health + tp.Shields
		if tp.Alive && prevAlive && (prevHP-currHP) > 0.01 {
			dealerIdx := findRecentDealerInDamageTable(ip, ctx.Tick)
			payload := map[string]any{
				"event_type": scraper.EventDamage,
				"receiver":   idx,
				"amount":     prevHP - currHP,
			}
			if dealerIdx >= 0 {
				payload["dealer"] = dealerIdx
			}
			out = append(out, ctx.emit(payload))
		}

		// --- melee ---
		// Fires when melee_damage_tick equals melee_remaining (both non-zero)
		// and melee_remaining changed since the previous tick.
		if ip.MeleeDamageTick > 0 && ip.MeleeDamageTick == ip.MeleeRemaining {
			prevMeleeRem := ctx.State.PrevMeleeRemaining[idx]
			if ip.MeleeRemaining != prevMeleeRem {
				victimIdx := findMeleeVictim(idx, ctx.Result.InternalPlayers, ctx.Tick)
				payload := map[string]any{
					"event_type": scraper.EventMelee,
					"player":     idx,
				}
				if victimIdx >= 0 {
					payload["victim"] = victimIdx
				}
				out = append(out, ctx.emit(payload))
			}
		}
	}
	return out
}

func updateDamagePrev(state *scraper.TickState, result scraper.TickResult) {
	for _, tp := range result.Payload.Players {
		state.PrevHealth[tp.Index] = tp.Health
		state.PrevShields[tp.Index] = tp.Shields
	}
	for _, ip := range result.InternalPlayers {
		state.PrevMeleeRemaining[ip.Index] = ip.MeleeRemaining
	}
}

func init() {
	RegisterDetector(detectDamage)
	RegisterUpdater(updateDamagePrev)
}
