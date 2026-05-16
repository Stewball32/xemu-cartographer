package manager

import (
	wsiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/websocket"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/capture"
	"github.com/Stewball32/xemu-cartographer/internal/websocket/rooms"
)

// shouldRead reports whether the runner should produce envelopes for
// (instance, class) on this poll. It unions the two demand sources
// described in atlas/new_json/04-ground-up-rebuild.md §5:
//
//	shouldRead = (NOT capture.never)
//	          AND (capture.always OR WS room has subscribers)
//
// `never` is a hard cap: it wins over WS subscribers and over `always`
// (Resolve picks one row; if `mode = never`, Demands() is false).
//
// A nil ws (test harness, pre-startup) is treated as permissive — the
// caller is not yet plugged into the hub, so we don't want to silently
// drop reads. policies may be nil; Resolve falls back to DefaultPolicy
// in that case (mode auto, no demand), so shouldRead reduces to
// "WS has subscribers" — i.e. behaves as if capture policy doesn't exist.
func shouldRead(instance, class string, policies []capture.Policy, ws wsiface.Rooms) bool {
	pol := capture.Resolve(policies, instance, class)
	if pol.IsHardCap() {
		return false
	}
	if pol.Demands() {
		return true
	}
	if ws == nil {
		return true
	}
	room, err := rooms.RoomForInstanceClass(instance, class)
	if err != nil {
		return false
	}
	return ws.RoomHasMembers(room)
}

// shouldReadAny returns true if any of the listed classes wants reads
// for this instance. Used for read paths that produce data for multiple
// classes from one source — e.g. ReadTick feeds the tick / objects /
// debug classes, so the runner should perform that read iff *any* of
// the three has demand. A `never` on one class still allows the read
// when another class wants it; the per-class gate inside broadcastPoll
// suppresses the individual broadcast.
func shouldReadAny(instance string, classes []string, policies []capture.Policy, ws wsiface.Rooms) bool {
	for _, class := range classes {
		if shouldRead(instance, class, policies, ws) {
			return true
		}
	}
	return false
}
