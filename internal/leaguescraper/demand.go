package leaguescraper

import (
	"github.com/xemu-cartographer/xc-scraper/runner"
	"github.com/xemu-cartographer/xc-scraper/wire"
	"github.com/xemu-cartographer/xemu-cartographer/internal/guards"
)

// wsDemand is the league server's runner.Demand: (instance, class) is
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

// NewDemand returns the WebSocket-room-backed runner.Demand for svc.
func NewDemand(svc *guards.Services) runner.Demand {
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
