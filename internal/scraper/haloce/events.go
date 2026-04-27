package haloce

import (
	"encoding/json"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// DetectEvents compares the current scraper.TickResult against scraper.TickState and returns all
// events that fired this tick. It also updates scraper.TickState for the next comparison.
func DetectEvents(tick uint32, instance string, snap scraper.SnapshotPayload, result scraper.TickResult, state *scraper.TickState) []scraper.Envelope {
	var events []scraper.Envelope
	emit := func(eventType string, payload any) {
		b, _ := json.Marshal(payload)
		events = append(events, scraper.Envelope{
			Type:     "event",
			Instance: instance,
			Tick:     tick,
			Payload:  b,
		})
		_ = eventType // eventType is embedded in payload
	}

	// Build player lookup maps from snapshot (for team info) and tick (for index).
	snapshotByIdx := make(map[int]scraper.SnapshotPlayer, len(snap.Players))
	for _, p := range snap.Players {
		snapshotByIdx[p.Index] = p
	}

	// -------------------------------------------------------------------
	// Per-player events
	// -------------------------------------------------------------------
	for _, ip := range result.InternalPlayers {
		idx := ip.Index
		tp := findTickPlayer(result.Payload.Players, idx)

		// game_start / game_end are handled by the poll loop, not here.

		// --- player_quit ---
		if prev, ok := state.PrevQuit[idx]; ok {
			if prev == 0 && ip.QuitFlag == 1 {
				emit(scraper.EventPlayerQuit, map[string]any{"event_type": scraper.EventPlayerQuit, "player": idx})
			}
		}

		// --- death ---
		prevAlive := state.PrevAlive[idx]
		if prevAlive && !tp.Alive {
			emit(scraper.EventDeath, map[string]any{
				"event_type":       scraper.EventDeath,
				"player":           idx,
				"respawn_in_ticks": ip.RespawnTimer,
			})

			// --- kill (find who killed this player) ---
			killerIdx := -1
			for _, other := range result.InternalPlayers {
				if other.Index == idx {
					continue
				}
				prevKills := state.PrevKills[other.Index]
				if other.Kills > prevKills {
					killerIdx = other.Index
					break
				}
			}
			// Also check damage table for kill attribution when kills haven't updated yet.
			if killerIdx < 0 {
				killerIdx = findKillerInDamageTable(ip, tick)
			}

			if killerIdx >= 0 {
				emit(scraper.EventKill, map[string]any{"event_type": scraper.EventKill, "killer": killerIdx, "victim": idx})

				// --- team_kill ---
				killerSnap, killerOk := snapshotByIdx[killerIdx]
				victimSnap, victimOk := snapshotByIdx[idx]
				if killerOk && victimOk && snap.IsTeamGame && killerSnap.Team == victimSnap.Team {
					emit(scraper.EventTeamKill, map[string]any{"event_type": scraper.EventTeamKill, "killer": killerIdx, "victim": idx})
				}

				// --- score ---
				killerIP := findInternal(result.InternalPlayers, killerIdx)
				if killerIP != nil {
					emit(scraper.EventScore, map[string]any{
						"event_type":  scraper.EventScore,
						"player":      killerIdx,
						"kills":       killerIP.Kills,
						"deaths":      killerIP.Deaths,
						"assists":     killerIP.Assists,
						"kill_streak": killerIP.KillStreak,
						"multikill":   killerIP.Multikill,
					})

					// --- multikill ---
					prevMK := state.PrevMultikill[killerIdx]
					if killerIP.Multikill > prevMK && killerIP.Multikill > 1 {
						emit(scraper.EventMultikill, map[string]any{
							"event_type": scraper.EventMultikill,
							"player":     killerIdx,
							"count":      killerIP.Multikill,
						})
					}

					// --- kill_streak ---
					prevKS := state.PrevKillStreak[killerIdx]
					if killerIP.KillStreak > prevKS {
						emit(scraper.EventKillStreak, map[string]any{
							"event_type": scraper.EventKillStreak,
							"player":     killerIdx,
							"count":      killerIP.KillStreak,
						})
					}
				}
			}
		}

		// --- spawn ---
		if !prevAlive && tp.Alive {
			emit(scraper.EventSpawn, map[string]any{
				"event_type": scraper.EventSpawn,
				"player":     idx,
				"x":          tp.X,
				"y":          tp.Y,
				"z":          tp.Z,
			})
		}

		// --- damage ---
		prevHP := state.PrevHealth[idx] + state.PrevShields[idx]
		currHP := tp.Health + tp.Shields
		if tp.Alive && prevAlive && (prevHP-currHP) > 0.01 {
			dealerIdx := findRecentDealerInDamageTable(ip, tick)
			payload := map[string]any{
				"event_type": scraper.EventDamage,
				"receiver":   idx,
				"amount":     prevHP - currHP,
			}
			if dealerIdx >= 0 {
				payload["dealer"] = dealerIdx
			}
			emit(scraper.EventDamage, payload)
		}

		// --- melee ---
		// Fires when melee_damage_tick equals melee_remaining (both non-zero) and
		// melee_remaining changed since last tick (animation advanced).
		if ip.MeleeDamageTick > 0 && ip.MeleeDamageTick == ip.MeleeRemaining {
			prevMeleeRem := state.PrevMeleeRemaining[idx]
			if ip.MeleeRemaining != prevMeleeRem {
				victimIdx := findMeleeVictim(idx, result.InternalPlayers, tick)
				payload := map[string]any{"event_type": scraper.EventMelee, "player": idx}
				if victimIdx >= 0 {
					payload["victim"] = victimIdx
				}
				emit(scraper.EventMelee, payload)
			}
		}

		// --- grenade_thrown ---
		prevFrags := state.PrevFrags[idx]
		if tp.Alive && tp.Frags < prevFrags {
			emit(scraper.EventGrenadeThrown, map[string]any{
				"event_type":      scraper.EventGrenadeThrown,
				"player":          idx,
				"kind":            "frag",
				"frags_remaining": tp.Frags,
			})
		}
		prevPlasmas := state.PrevPlasmas[idx]
		if tp.Alive && tp.Plasmas < prevPlasmas {
			emit(scraper.EventGrenadeThrown, map[string]any{
				"event_type":        scraper.EventGrenadeThrown,
				"player":            idx,
				"kind":              "plasma",
				"plasmas_remaining": tp.Plasmas,
			})
		}

		// --- powerup_picked_up / powerup_expired ---
		prevCamo := state.PrevHasCamo[idx]
		if !prevCamo && tp.HasCamo {
			emit(scraper.EventPowerupPickup, map[string]any{
				"event_type": scraper.EventPowerupPickup,
				"player":     idx,
				"kind":       "active_camouflage",
			})
		}
		if prevCamo && !tp.HasCamo && tp.Alive {
			emit(scraper.EventPowerupExpired, map[string]any{
				"event_type": scraper.EventPowerupExpired,
				"player":     idx,
				"kind":       "active_camouflage",
			})
		}
		prevOS := state.PrevHasOvershield[idx]
		if !prevOS && tp.HasOvershield {
			emit(scraper.EventPowerupPickup, map[string]any{
				"event_type": scraper.EventPowerupPickup,
				"player":     idx,
				"kind":       "overshield",
			})
		}
		if prevOS && !tp.HasOvershield && tp.Alive {
			emit(scraper.EventPowerupExpired, map[string]any{
				"event_type": scraper.EventPowerupExpired,
				"player":     idx,
				"kind":       "overshield",
			})
		}

		// --- vehicle_entered / vehicle_exited ---
		// Guard with prevAlive && tp.Alive: dead-biped memory reads garbage into
		// ParentObject, causing false enter/exit events on death and respawn ticks.
		prevParent := state.PrevParentObject[idx]
		if prevAlive && tp.Alive && prevParent == HandleEmpty && ip.ParentObject != HandleEmpty {
			emit(scraper.EventVehicleEntered, map[string]any{
				"event_type":     scraper.EventVehicleEntered,
				"player":         idx,
				"vehicle_handle": ip.ParentObject & HandleIndexMask,
			})
		}
		if prevAlive && tp.Alive && prevParent != HandleEmpty && ip.ParentObject == HandleEmpty {
			emit(scraper.EventVehicleExited, map[string]any{
				"event_type": scraper.EventVehicleExited,
				"player":     idx,
			})
		}

		// --- item_depleted (per weapon slot) ---
		for _, w := range tp.Weapons {
			key := idx*4 + w.Slot
			if w.IsEnergy {
				prev := state.PrevWeaponEnergy[key]
				cur := float32(0)
				if w.Charge != nil {
					cur = *w.Charge
				}
				if prev > 0.01 && cur <= 0.01 {
					emit(scraper.EventItemDepleted, map[string]any{
						"event_type": scraper.EventItemDepleted,
						"player":     idx,
						"tag":        w.Tag,
						"kind":       "energy",
					})
				}
			} else if w.AmmoMag != nil && w.AmmoPack != nil {
				prevAmmo := state.PrevWeaponAmmo[key]
				if prevAmmo > 0 && *w.AmmoMag == 0 && *w.AmmoPack == 0 {
					emit(scraper.EventItemDepleted, map[string]any{
						"event_type": scraper.EventItemDepleted,
						"player":     idx,
						"tag":        w.Tag,
						"kind":       "ammo",
					})
				}
			}
		}
	}

	// -------------------------------------------------------------------
	// Power item events (compare current status to previous)
	// -------------------------------------------------------------------
	spawnMap := make(map[int]scraper.PowerItemSpawn, len(snap.PowerItemSpawns))
	for _, s := range snap.PowerItemSpawns {
		spawnMap[s.SpawnID] = s
	}

	for _, pi := range result.Payload.PowerItems {
		prevStatus := state.PrevPowerItemStatus[pi.SpawnID]
		spawn, hasSpawn := spawnMap[pi.SpawnID]
		if !hasSpawn {
			continue
		}

		// item_picked_up: world → held
		if prevStatus == "world" && pi.Status == "held" && pi.HeldBy != nil {
			emit(scraper.EventItemPickedUp, map[string]any{
				"event_type": scraper.EventItemPickedUp,
				"spawn_id":   pi.SpawnID,
				"player":     *pi.HeldBy,
				"tag":        spawn.Tag,
			})
		}

		// item_dropped: held → world
		if prevStatus == "held" && pi.Status == "world" {
			prevHolder := state.PrevPowerItemHeldBy[pi.SpawnID]
			payload := map[string]any{
				"event_type": scraper.EventItemDropped,
				"spawn_id":   pi.SpawnID,
				"tag":        spawn.Tag,
			}
			if prevHolder >= 0 {
				payload["player"] = prevHolder
			}
			if pi.WorldPos != nil {
				payload["x"] = pi.WorldPos.X
				payload["y"] = pi.WorldPos.Y
				payload["z"] = pi.WorldPos.Z
			}
			emit(scraper.EventItemDropped, payload)
		}

		// item_spawned: respawning → world
		if prevStatus == "respawning" && pi.Status == "world" {
			emit(scraper.EventItemSpawned, map[string]any{
				"event_type": scraper.EventItemSpawned,
				"spawn_id":   pi.SpawnID,
				"tag":        spawn.Tag,
			})
		}
	}

	// -------------------------------------------------------------------
	// Update scraper.TickState
	// -------------------------------------------------------------------
	UpdateTickState(state, result)

	return events
}

// UpdateTickState copies current tick values into scraper.TickState for the next comparison.
func UpdateTickState(state *scraper.TickState, result scraper.TickResult) {
	for _, tp := range result.Payload.Players {
		state.PrevAlive[tp.Index] = tp.Alive
		state.PrevHealth[tp.Index] = tp.Health
		state.PrevShields[tp.Index] = tp.Shields
		state.PrevFrags[tp.Index] = tp.Frags
		state.PrevPlasmas[tp.Index] = tp.Plasmas
		state.PrevHasCamo[tp.Index] = tp.HasCamo
		state.PrevHasOvershield[tp.Index] = tp.HasOvershield

		for _, w := range tp.Weapons {
			key := tp.Index*4 + w.Slot
			if w.IsEnergy && w.Charge != nil {
				state.PrevWeaponEnergy[key] = *w.Charge
			} else if !w.IsEnergy && w.AmmoMag != nil {
				state.PrevWeaponAmmo[key] = *w.AmmoMag
			}
		}
	}

	for _, ip := range result.InternalPlayers {
		idx := ip.Index
		state.PrevKills[idx] = ip.Kills
		state.PrevDeaths[idx] = ip.Deaths
		state.PrevAssists[idx] = ip.Assists
		state.PrevTeamKills[idx] = ip.TeamKills
		state.PrevSuicides[idx] = ip.Suicides
		state.PrevKillStreak[idx] = ip.KillStreak
		state.PrevMultikill[idx] = ip.Multikill
		state.PrevQuit[idx] = ip.QuitFlag
		state.PrevParentObject[idx] = ip.ParentObject
		state.PrevMeleeRemaining[idx] = ip.MeleeRemaining
		state.PrevWeaponSlots[idx] = ip.WeaponSlots
	}

	for _, pi := range result.Payload.PowerItems {
		state.PrevPowerItemStatus[pi.SpawnID] = pi.Status
		if pi.HeldBy != nil {
			state.PrevPowerItemHeldBy[pi.SpawnID] = *pi.HeldBy
		} else {
			state.PrevPowerItemHeldBy[pi.SpawnID] = -1
		}
	}
}

// -------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------

func findTickPlayer(players []scraper.TickPlayer, index int) scraper.TickPlayer {
	for _, p := range players {
		if p.Index == index {
			return p
		}
	}
	return scraper.TickPlayer{Index: index}
}

func findInternal(players []scraper.InternalPlayerState, index int) *scraper.InternalPlayerState {
	for i := range players {
		if players[i].Index == index {
			return &players[i]
		}
	}
	return nil
}

// findKillerInDamageTable finds which player dealt the killing blow by scanning
// the victim's damage table for the most recent entry near the current tick.
func findKillerInDamageTable(ip scraper.InternalPlayerState, tick uint32) int {
	best := uint32(0)
	killerIdx := -1
	for _, e := range ip.DamageTable {
		if e.DamageTime == DamageEmptySentinel {
			continue
		}
		// Accept entries within 5 ticks of current tick.
		if tick >= e.DamageTime && tick-e.DamageTime <= 5 {
			if e.DamageTime >= best {
				best = e.DamageTime
				killerIdx = int(e.DealerPlrHandle & HandleIndexMask)
			}
		}
	}
	return killerIdx
}

// findRecentDealerInDamageTable finds the most recent dealer for a damage event.
func findRecentDealerInDamageTable(ip scraper.InternalPlayerState, tick uint32) int {
	best := uint32(0)
	dealerIdx := -1
	for _, e := range ip.DamageTable {
		if e.DamageTime == DamageEmptySentinel {
			continue
		}
		if tick >= e.DamageTime && tick-e.DamageTime <= 2 {
			if e.DamageTime >= best {
				best = e.DamageTime
				dealerIdx = int(e.DealerPlrHandle & HandleIndexMask)
			}
		}
	}
	return dealerIdx
}

// findMeleeVictim finds which player was hit by player dealerIdx's melee this tick.
func findMeleeVictim(dealerIdx int, players []scraper.InternalPlayerState, tick uint32) int {
	for _, p := range players {
		if p.Index == dealerIdx {
			continue
		}
		for _, e := range p.DamageTable {
			if e.DamageTime == DamageEmptySentinel {
				continue
			}
			if tick >= e.DamageTime && tick-e.DamageTime <= 2 {
				if int(e.DealerPlrHandle&HandleIndexMask) == dealerIdx {
					return p.Index
				}
			}
		}
	}
	return -1
}
