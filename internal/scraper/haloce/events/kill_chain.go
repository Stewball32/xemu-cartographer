package events

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// detectKillChain handles every event hanging off the player's Alive flip:
// death, the killer's kill (with team_kill / score / multikill / kill_streak
// follow-ons), and respawn. Grouped together because they share the
// prev-vs-current Alive transition trigger and because kill attribution
// requires comparing kill counters across all players in this single tick.
func detectKillChain(ctx *Context) []scraper.Envelope {
	var out []scraper.Envelope
	snapByIdx := gamePlayerByIndex(ctx.Snap)

	for _, ip := range ctx.Result.InternalPlayers {
		idx := ip.Index
		tp := findTickPlayer(ctx.Result.Payload.Players, idx)
		prevAlive := ctx.State.PrevAlive[idx]

		// --- death ---
		if prevAlive && !tp.Alive {
			out = append(out, ctx.emit(map[string]any{
				"event_type":       scraper.EventDeath,
				"player":           idx,
				"respawn_in_ticks": ip.RespawnTimer,
			}))

			// --- kill (find who killed this player) ---
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

			if killerIdx >= 0 {
				out = append(out, ctx.emit(map[string]any{
					"event_type": scraper.EventKill,
					"killer":     killerIdx,
					"victim":     idx,
				}))

				// --- team_kill ---
				killerSnap, killerOk := snapByIdx[killerIdx]
				victimSnap, victimOk := snapByIdx[idx]
				if killerOk && victimOk && ctx.Snap.IsTeamGame && killerSnap.Team == victimSnap.Team {
					out = append(out, ctx.emit(map[string]any{
						"event_type": scraper.EventTeamKill,
						"killer":     killerIdx,
						"victim":     idx,
					}))
				}

				// --- score / multikill / kill_streak ---
				killerIP := findInternal(ctx.Result.InternalPlayers, killerIdx)
				if killerIP != nil {
					out = append(out, ctx.emit(map[string]any{
						"event_type":  scraper.EventScore,
						"player":      killerIdx,
						"kills":       killerIP.Kills,
						"deaths":      killerIP.Deaths,
						"assists":     killerIP.Assists,
						"kill_streak": killerIP.KillStreak,
						"multikill":   killerIP.Multikill,
					}))

					prevMK := ctx.State.PrevMultikill[killerIdx]
					if killerIP.Multikill > prevMK && killerIP.Multikill > 1 {
						out = append(out, ctx.emit(map[string]any{
							"event_type": scraper.EventMultikill,
							"player":     killerIdx,
							"count":      killerIP.Multikill,
						}))
					}
					prevKS := ctx.State.PrevKillStreak[killerIdx]
					if killerIP.KillStreak > prevKS {
						out = append(out, ctx.emit(map[string]any{
							"event_type": scraper.EventKillStreak,
							"player":     killerIdx,
							"count":      killerIP.KillStreak,
						}))
					}
				}
			}
		}

		// --- spawn ---
		if !prevAlive && tp.Alive {
			out = append(out, ctx.emit(map[string]any{
				"event_type": scraper.EventSpawn,
				"player":     idx,
				"x":          tp.X,
				"y":          tp.Y,
				"z":          tp.Z,
			}))
		}
	}
	return out
}

// updateKillChainPrev records this tick's Alive / kill-counter / multikill /
// kill-streak values so the next tick's detector can diff against them.
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
