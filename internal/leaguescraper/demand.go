package leaguescraper

import (
	"github.com/Stewball32/xemu-cartographer/internal/guards"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/manager"
	"github.com/xemu-cartographer/xc-scraper/wire"
)

// wsDemand is the league server's manager.Demand: (instance, class) is
// wanted when the per-class WebSocket room (host:<inst>:<class>) has at
// least one member. This is the subscriber half of the manager's shouldRead
// formula, lifted out verbatim from the pre-port demand.go:
//
//	room, err := RoomForInstanceClass(instance, class); err → false
//	no hub yet (svc.WS == nil)                              → true (permissive)
//	otherwise                                               → WS.RoomHasMembers(room)
//
// svc.WS is read at call time (see wsEmitter) — before the hub exists the
// answer is permissive, matching the manager's old nil-ws behaviour.
type wsDemand struct {
	svc *guards.Services
}

// NewDemand returns the WebSocket-room-backed manager.Demand for svc.
func NewDemand(svc *guards.Services) manager.Demand {
	return &wsDemand{svc: svc}
}

func (d *wsDemand) Wants(instance, class string) bool {
	room, err := wire.RoomForInstanceClass(instance, class)
	if err != nil {
		return false
	}
	if d == nil || d.svc == nil || d.svc.WS == nil {
		return true
	}
	return d.svc.WS.RoomHasMembers(room)
}
