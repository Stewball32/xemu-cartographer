package events

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// detectItem emits the four item-related events:
//   - item_picked_up:   power-item world → held
//   - item_dropped:     power-item held → world
//   - item_spawned:     power-item respawning → world
//   - item_depleted:    per-player weapon ammo / energy zero-crossing
//
// Power-item events compare the current PowerItems status array against
// state.PrevPowerItemStatus / PrevPowerItemHeldBy. Item-depleted is per
// player-weapon-slot and uses PrevWeaponAmmo / PrevWeaponEnergy keyed on
// player_index*4 + slot.
func detectItem(ctx *Context) []scraper.Envelope {
	var out []scraper.Envelope

	// --- item_depleted (per weapon slot) ---
	for _, ip := range ctx.Result.InternalPlayers {
		idx := ip.Index
		tp := findTickPlayer(ctx.Result.Payload.Players, idx)
		for _, w := range tp.Weapons {
			key := idx*4 + w.Slot
			if w.IsEnergy {
				prev := ctx.State.PrevWeaponEnergy[key]
				cur := float32(0)
				if w.Charge != nil {
					cur = *w.Charge
				}
				if prev > 0.01 && cur <= 0.01 {
					out = append(out, ctx.emit(map[string]any{
						"event_type": scraper.EventItemDepleted,
						"player":     idx,
						"tag":        w.Tag,
						"kind":       "energy",
					}))
				}
			} else if w.AmmoMag != nil && w.AmmoPack != nil {
				prevAmmo := ctx.State.PrevWeaponAmmo[key]
				if prevAmmo > 0 && *w.AmmoMag == 0 && *w.AmmoPack == 0 {
					out = append(out, ctx.emit(map[string]any{
						"event_type": scraper.EventItemDepleted,
						"player":     idx,
						"tag":        w.Tag,
						"kind":       "ammo",
					}))
				}
			}
		}
	}

	// --- power-item world transitions (picked_up / dropped / spawned) ---
	spawnMap := make(map[int]scraper.PowerItemSpawn, len(ctx.Snap.PowerItemSpawns))
	for _, s := range ctx.Snap.PowerItemSpawns {
		spawnMap[s.SpawnID] = s
	}

	for _, pi := range ctx.Result.Payload.PowerItems {
		prevStatus := ctx.State.PrevPowerItemStatus[pi.SpawnID]
		spawn, hasSpawn := spawnMap[pi.SpawnID]
		if !hasSpawn {
			continue
		}

		// item_picked_up: world → held
		if prevStatus == "world" && pi.Status == "held" && pi.HeldBy != nil {
			out = append(out, ctx.emit(map[string]any{
				"event_type": scraper.EventItemPickedUp,
				"spawn_id":   pi.SpawnID,
				"player":     *pi.HeldBy,
				"tag":        spawn.Tag,
			}))
		}

		// item_dropped: held → world
		if prevStatus == "held" && pi.Status == "world" {
			prevHolder := ctx.State.PrevPowerItemHeldBy[pi.SpawnID]
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
			out = append(out, ctx.emit(payload))
		}

		// item_spawned: respawning → world
		if prevStatus == "respawning" && pi.Status == "world" {
			out = append(out, ctx.emit(map[string]any{
				"event_type": scraper.EventItemSpawned,
				"spawn_id":   pi.SpawnID,
				"tag":        spawn.Tag,
			}))
		}
	}
	return out
}

func updateItemPrev(state *scraper.TickState, result scraper.TickResult) {
	// Weapon ammo / energy per slot (player_index*4 + slot).
	for _, tp := range result.Payload.Players {
		for _, w := range tp.Weapons {
			key := tp.Index*4 + w.Slot
			if w.IsEnergy && w.Charge != nil {
				state.PrevWeaponEnergy[key] = *w.Charge
			} else if !w.IsEnergy && w.AmmoMag != nil {
				state.PrevWeaponAmmo[key] = *w.AmmoMag
			}
		}
	}
	// Power-item status / held-by per spawn.
	for _, pi := range result.Payload.PowerItems {
		state.PrevPowerItemStatus[pi.SpawnID] = pi.Status
		if pi.HeldBy != nil {
			state.PrevPowerItemHeldBy[pi.SpawnID] = *pi.HeldBy
		} else {
			state.PrevPowerItemHeldBy[pi.SpawnID] = -1
		}
	}
}

func init() {
	RegisterDetector(detectItem)
	RegisterUpdater(updateItemPrev)
}
