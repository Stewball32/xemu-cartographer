package events

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// detectItem emits the four item-related events under the v2 consolidated
// taxonomy:
//
//	player_update kind=item_picked_up  power-item world → held
//	player_update kind=item_dropped    power-item held → world
//	game_update   kind=item_spawned    power-item respawning → world (no player)
//	player_update kind=item_depleted   per-player weapon ammo/energy zero
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
		player := playerRefByIndex(ctx.Snap, idx)
		for _, w := range tp.Weapons {
			key := idx*4 + w.Slot
			if w.IsEnergy {
				prev := ctx.State.PrevWeaponEnergy[key]
				cur := float32(0)
				if w.Charge != nil {
					cur = *w.Charge
				}
				if prev > 0.01 && cur <= 0.01 {
					out = append(out, ctx.emitPlayerUpdate(scraper.PlayerUpdateEvent{
						Kind:   scraper.PlayerUpdateKindItemDepleted,
						Player: player,
						Item:   &scraper.ItemRef{Tag: w.Tag},
						Cause:  scraper.ItemDepletionEnergy,
					}))
				}
			} else if w.AmmoMag != nil && w.AmmoPack != nil {
				prevAmmo := ctx.State.PrevWeaponAmmo[key]
				if prevAmmo > 0 && *w.AmmoMag == 0 && *w.AmmoPack == 0 {
					out = append(out, ctx.emitPlayerUpdate(scraper.PlayerUpdateEvent{
						Kind:   scraper.PlayerUpdateKindItemDepleted,
						Player: player,
						Item:   &scraper.ItemRef{Tag: w.Tag},
						Cause:  scraper.ItemDepletionAmmo,
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
		spawnID := pi.SpawnID

		// item_picked_up: world → held
		if prevStatus == "world" && pi.Status == "held" && pi.HeldBy != nil {
			tp := findTickPlayer(ctx.Result.Payload.Players, *pi.HeldBy)
			pos := vec3FromTickPlayer(tp)
			out = append(out, ctx.emitPlayerUpdate(scraper.PlayerUpdateEvent{
				Kind:   scraper.PlayerUpdateKindItemPickedUp,
				Player: playerRefByIndex(ctx.Snap, *pi.HeldBy),
				Pos:    &pos,
				Item:   &scraper.ItemRef{SpawnID: &spawnID, Tag: spawn.Tag},
			}))
		}

		// item_dropped: held → world. pos is where the item landed.
		if prevStatus == "held" && pi.Status == "world" {
			prevHolder := ctx.State.PrevPowerItemHeldBy[pi.SpawnID]
			event := scraper.PlayerUpdateEvent{
				Kind: scraper.PlayerUpdateKindItemDropped,
				Item: &scraper.ItemRef{SpawnID: &spawnID, Tag: spawn.Tag},
			}
			if prevHolder >= 0 {
				event.Player = playerRefByIndex(ctx.Snap, prevHolder)
			}
			if pi.WorldPos != nil {
				event.Pos = vec3Ptr(pi.WorldPos.X, pi.WorldPos.Y, pi.WorldPos.Z)
			}
			out = append(out, ctx.emitPlayerUpdate(event))
		}

		// item_spawned: respawning → world. No player; this is a world event.
		if prevStatus == "respawning" && pi.Status == "world" {
			event := scraper.GameUpdateEvent{
				Kind: scraper.GameUpdateKindItemSpawned,
				Item: &scraper.ItemRef{SpawnID: &spawnID, Tag: spawn.Tag},
			}
			if pi.WorldPos != nil {
				event.Pos = vec3Ptr(pi.WorldPos.X, pi.WorldPos.Y, pi.WorldPos.Z)
			} else {
				event.Pos = vec3Ptr(spawn.X, spawn.Y, spawn.Z)
			}
			out = append(out, ctx.emitGameUpdate(event))
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
