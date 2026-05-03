package events

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// detectVehicle emits vehicle_entered / vehicle_exited based on the
// ParentObject diff. Both transitions are gated on prevAlive && tp.Alive
// because dead bipeds read garbage into ParentObject and would otherwise
// fire spurious enter/exit events on death and respawn ticks.
func detectVehicle(ctx *Context) []scraper.Envelope {
	var out []scraper.Envelope

	for _, ip := range ctx.Result.InternalPlayers {
		idx := ip.Index
		tp := findTickPlayer(ctx.Result.Payload.Players, idx)
		prevAlive := ctx.State.PrevAlive[idx]
		if !(prevAlive && tp.Alive) {
			continue
		}
		prevParent := ctx.State.PrevParentObject[idx]

		if prevParent == handleEmpty && ip.ParentObject != handleEmpty {
			out = append(out, ctx.emit(map[string]any{
				"event_type":     scraper.EventVehicleEntered,
				"player":         idx,
				"vehicle_handle": ip.ParentObject & handleIndexMask,
			}))
		}
		if prevParent != handleEmpty && ip.ParentObject == handleEmpty {
			out = append(out, ctx.emit(map[string]any{
				"event_type": scraper.EventVehicleExited,
				"player":     idx,
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
