package events

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// detectVehicle emits player_update events (kind=vehicle_entered /
// vehicle_exited) based on the ParentObject diff. Both transitions are
// gated on prevAlive && tp.Alive because dead bipeds read garbage into
// ParentObject and would otherwise fire spurious enter/exit events on
// death and respawn ticks.
//
// Seat detection (driver/passenger/gunner) is not yet wired — requires
// walking the vehicle's seat-anchor table. For now Seat is left empty;
// improvement tracked separately.
func detectVehicle(ctx *Context) []scraper.Envelope {
	var out []scraper.Envelope

	for _, ip := range ctx.Result.InternalPlayers {
		idx := ip.Index
		tp := findTickPlayer(ctx.Result.Payload.Players, idx)
		prevAlive := ctx.State.PrevAlive[idx]
		if !prevAlive || !tp.Alive {
			continue
		}
		prevParent := ctx.State.PrevParentObject[idx]

		if prevParent == handleEmpty && ip.ParentObject != handleEmpty {
			vehicleID := ip.ParentObject & handleIndexMask
			out = append(out, ctx.emitPlayerUpdate(scraper.PlayerUpdateEvent{
				Kind:    scraper.PlayerUpdateKindVehicleEntered,
				Player:  playerRefByIndex(ctx.Snap, idx),
				Pos:     vec3Ptr(tp.X, tp.Y, tp.Z),
				Vehicle: &scraper.VehicleRef{ObjectID: vehicleID},
			}))
		}
		if prevParent != handleEmpty && ip.ParentObject == handleEmpty {
			// Vehicle ref left nil — at exit time we don't know which vehicle
			// they were in (parent already cleared). Could track via state if
			// needed.
			out = append(out, ctx.emitPlayerUpdate(scraper.PlayerUpdateEvent{
				Kind:   scraper.PlayerUpdateKindVehicleExited,
				Player: playerRefByIndex(ctx.Snap, idx),
				Pos:    vec3Ptr(tp.X, tp.Y, tp.Z),
			}))
		}
	}
	return out
}

func updateVehiclePrev(state *scraper.TickState, result scraper.TickResult) {
	for _, ip := range result.InternalPlayers {
		state.PrevParentObject[ip.Index] = ip.ParentObject
	}
}

func init() {
	RegisterDetector(detectVehicle)
	RegisterUpdater(updateVehiclePrev)
}
