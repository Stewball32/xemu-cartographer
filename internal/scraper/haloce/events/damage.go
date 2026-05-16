package events

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// detectDamage emits damage events under the v2 consolidated taxonomy:
//
//	damage kind=hit    HP-loss tick (gunfire, splash, fall, etc.)
//	damage kind=melee  melee strike
//
// Both share the damage-table helpers and the dealer→victim positional
// enrichment.
//
// Weapon attribution and melee damage amount aren't yet wired through —
// they require additional lookups (held weapon at strike time, damage
// from the matching damage-table entry). Today these emit with
// Weapon="" / Amount=0 for melee; improvement tracked separately.
func detectDamage(ctx *Context) []scraper.Envelope {
	var out []scraper.Envelope

	for _, ip := range ctx.Result.InternalPlayers {
		idx := ip.Index
		tp := findTickPlayer(ctx.Result.Payload.Players, idx)
		prevAlive := ctx.State.PrevAlive[idx]

		// --- hit damage (HP loss) ---
		prevHP := ctx.State.PrevHealth[idx] + ctx.State.PrevShields[idx]
		currHP := tp.Health + tp.Shields
		if tp.Alive && prevAlive && (prevHP-currHP) > 0.01 {
			dealerIdx := findRecentDealerInDamageTable(ip, ctx.Tick)
			event := scraper.DamageEvent{
				Kind:      scraper.DamageKindHit,
				Victim:    playerRefByIndex(ctx.Snap, idx),
				VictimPos: vec3FromTickPlayer(tp),
				Amount:    prevHP - currHP,
			}
			if dealerIdx >= 0 {
				dealer := playerRefByIndex(ctx.Snap, dealerIdx)
				dealerTP := findTickPlayer(ctx.Result.Payload.Players, dealerIdx)
				dealerPos := vec3FromTickPlayer(dealerTP)
				event.Dealer = &dealer
				event.DealerPos = &dealerPos
			}
			out = append(out, ctx.emitDamage(event))
		}

		// --- melee strike ---
		// Fires when melee_damage_tick equals melee_remaining (both non-zero)
		// and melee_remaining changed since the previous tick.
		if ip.MeleeDamageTick > 0 && ip.MeleeDamageTick == ip.MeleeRemaining {
			prevMeleeRem := ctx.State.PrevMeleeRemaining[idx]
			if ip.MeleeRemaining != prevMeleeRem {
				victimIdx := findMeleeVictim(idx, ctx.Result.InternalPlayers, ctx.Tick)
				if victimIdx < 0 {
					continue
				}
				victimTP := findTickPlayer(ctx.Result.Payload.Players, victimIdx)
				dealer := playerRefByIndex(ctx.Snap, idx)
				dealerPos := vec3FromTickPlayer(tp)
				out = append(out, ctx.emitDamage(scraper.DamageEvent{
					Kind:      scraper.DamageKindMelee,
					Dealer:    &dealer,
					DealerPos: &dealerPos,
					Victim:    playerRefByIndex(ctx.Snap, victimIdx),
					VictimPos: vec3FromTickPlayer(victimTP),
				}))
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
