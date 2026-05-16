package events

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// detectKillChain handles every event hanging off the player's Alive flip:
// death (with attributed killer when known, including team-kill detection),
// the killer's score update (player_update kind=score), milestone medals
// (multikill / kill_streak), and respawn (player_update kind=spawn).
//
// Grouped together because they share the prev-vs-current Alive transition
// trigger and because kill attribution requires comparing kill counters
// across all players in this single tick.
//
// v2 consolidation: the v1 separate `kill` and `team_kill` events are
// folded into the death event's Killer + TeamKill / Cause fields. The v1
// `score` event becomes a player_update with kind=score; `multikill` and
// `kill_streak` become medals; `spawn` becomes a player_update.
//
// Weapon attribution on death is not yet wired through — death weapon
// requires correlating the damage-table entry with the killer's selected
// weapon at kill time. Today emits with Weapon="".
func detectKillChain(ctx *Context) []scraper.Envelope {
	var out []scraper.Envelope
	snapByIdx := gamePlayerByIndex(ctx.Snap)

	for _, ip := range ctx.Result.InternalPlayers {
		idx := ip.Index
		tp := findTickPlayer(ctx.Result.Payload.Players, idx)
		prevAlive := ctx.State.PrevAlive[idx]

		// --- death ---
		if prevAlive && !tp.Alive {
			// Find killer: prefer kill-counter advancement, fall back to
			// the damage-table's most-recent dealer.
			killerIdx := -1
			for _, other := range ctx.Result.InternalPlayers {
				if other.Index == idx {
					continue
				}
				prevKills := ctx.State.PrevKills[other.Index]
				if other.Kills > prevKills {
					killerIdx = other.Index
					break
				}
			}
			if killerIdx < 0 {
				killerIdx = findKillerInDamageTable(ip, ctx.Tick)
			}

			event := scraper.DeathEvent{
				Victim:         playerRefByIndex(ctx.Snap, idx),
				VictimPos:      vec3FromTickPlayer(tp),
				Cause:          scraper.DeathCauseUnknown,
				RespawnInTicks: ip.RespawnTimer,
			}

			if killerIdx >= 0 {
				killer := playerRefByIndex(ctx.Snap, killerIdx)
				killerTP := findTickPlayer(ctx.Result.Payload.Players, killerIdx)
				killerPos := vec3FromTickPlayer(killerTP)
				event.Killer = &killer
				event.KillerPos = &killerPos
				event.Cause = scraper.DeathCauseKill

				// team-kill detection
				killerSnap, killerOk := snapByIdx[killerIdx]
				victimSnap, victimOk := snapByIdx[idx]
				if killerOk && victimOk && ctx.Snap.IsTeamGame && killerSnap.Team == victimSnap.Team {
					event.TeamKill = true
					event.Cause = scraper.DeathCauseBetrayal
				}
			}
			out = append(out, ctx.emitDeath(event))

			// --- score + medals for the killer ---
			if killerIdx >= 0 {
				killerIP := findInternal(ctx.Result.InternalPlayers, killerIdx)
				if killerIP != nil {
					kills := killerIP.Kills
					deaths := killerIP.Deaths
					assists := killerIP.Assists
					killStreak := killerIP.KillStreak
					multikill := killerIP.Multikill
					out = append(out, ctx.emitPlayerUpdate(scraper.PlayerUpdateEvent{
						Kind:       scraper.PlayerUpdateKindScore,
						Player:     playerRefByIndex(ctx.Snap, killerIdx),
						Kills:      &kills,
						Deaths:     &deaths,
						Assists:    &assists,
						KillStreak: &killStreak,
						Multikill:  &multikill,
					}))

					prevMK := ctx.State.PrevMultikill[killerIdx]
					if killerIP.Multikill > prevMK && killerIP.Multikill > 1 {
						out = append(out, ctx.emitMedal(scraper.MedalEvent{
							Kind:   scraper.MedalKindMultikill,
							Player: playerRefByIndex(ctx.Snap, killerIdx),
							Count:  killerIP.Multikill,
						}))
					}
					prevKS := ctx.State.PrevKillStreak[killerIdx]
					if killerIP.KillStreak > prevKS {
						out = append(out, ctx.emitMedal(scraper.MedalEvent{
							Kind:   scraper.MedalKindKillStreak,
							Player: playerRefByIndex(ctx.Snap, killerIdx),
							Count:  killerIP.KillStreak,
						}))
					}
				}
			}
		}

		// --- spawn (dead → alive) ---
		if !prevAlive && tp.Alive {
			pos := vec3FromTickPlayer(tp)
			out = append(out, ctx.emitPlayerUpdate(scraper.PlayerUpdateEvent{
				Kind:   scraper.PlayerUpdateKindSpawn,
				Player: playerRefByIndex(ctx.Snap, idx),
				Pos:    &pos,
			}))
		}
	}
	return out
}

// updateKillChainPrev records this tick's Alive / kill-counter / multikill
// / kill-streak values so the next tick's detector can diff against them.
func updateKillChainPrev(state *scraper.TickState, result scraper.TickResult) {
	for _, tp := range result.Payload.Players {
		state.PrevAlive[tp.Index] = tp.Alive
	}
	for _, ip := range result.InternalPlayers {
		state.PrevKills[ip.Index] = ip.Kills
		state.PrevDeaths[ip.Index] = ip.Deaths
		state.PrevAssists[ip.Index] = ip.Assists
		state.PrevTeamKills[ip.Index] = ip.TeamKills
		state.PrevSuicides[ip.Index] = ip.Suicides
		state.PrevKillStreak[ip.Index] = ip.KillStreak
		state.PrevMultikill[ip.Index] = ip.Multikill
	}
}

func init() {
	RegisterDetector(detectKillChain)
	RegisterUpdater(updateKillChainPrev)
}
