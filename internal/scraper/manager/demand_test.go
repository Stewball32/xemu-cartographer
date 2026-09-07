package manager

import (
	"testing"

	"github.com/xemu-cartographer/xc-scraper/capture"
	"github.com/xemu-cartographer/xc-scraper/wire"
)

// stubDemand is a minimal Demand (ports.go) that reports a fixed set of
// per-class rooms as occupied, keyed the way the league Demand resolves
// them (wire.RoomForInstanceClass) so the assertions read like the WS-era
// room-membership ones.
type stubDemand struct {
	occupied map[string]bool
}

func (s *stubDemand) Wants(instance, class string) bool {
	room, err := wire.RoomForInstanceClass(instance, class)
	if err != nil {
		return false
	}
	return s.occupied[room]
}

// TestShouldReadAutoDefersToWS: with no capture rows (DefaultPolicy
// returns auto, default), the WS room membership decides — empty room
// → false, occupied → true. This is the pre-capture-policy baseline.
func TestShouldReadAutoDefersToWS(t *testing.T) {
	room, _ := wire.RoomForInstanceClass("pod-1", "tick")
	empty := &stubDemand{occupied: map[string]bool{}}
	full := &stubDemand{occupied: map[string]bool{room: true}}

	if shouldRead("pod-1", "tick", nil, empty) {
		t.Fatal("auto + empty room: want false")
	}
	if !shouldRead("pod-1", "tick", nil, full) {
		t.Fatal("auto + occupied room: want true")
	}
}

// TestShouldReadAlwaysOverridesEmptyWS: capture mode `always` forces
// reads even when no one is listening on the WS room — this is the
// capture-without-viewers case (e.g. tournament replay recording).
func TestShouldReadAlwaysOverridesEmptyWS(t *testing.T) {
	policies := []capture.Policy{
		{Instance: "*", Class: "tick", Mode: capture.ModeAlways},
	}
	ws := &stubDemand{occupied: map[string]bool{}}
	if !shouldRead("pod-1", "tick", policies, ws) {
		t.Fatal("always + empty room: want true")
	}
}

// TestShouldReadNeverHardCaps: mode `never` is the operator's "stop
// reading this class" switch — must beat WS subscribers and any
// `always` policy that would otherwise be reached. (Resolve picks one
// row by precedence; an exact never wins over wildcard always.)
func TestShouldReadNeverHardCaps(t *testing.T) {
	room, _ := wire.RoomForInstanceClass("pod-1", "debug")
	ws := &stubDemand{occupied: map[string]bool{room: true}}
	policies := []capture.Policy{
		{Instance: "*", Class: "debug", Mode: capture.ModeAlways},
		{Instance: "pod-1", Class: "debug", Mode: capture.ModeNever},
	}
	if shouldRead("pod-1", "debug", policies, ws) {
		t.Fatal("never + occupied room: want false (hard cap)")
	}
}

// TestShouldReadNilWSIsPermissive: callers without a hub (one-shot
// tests, pre-startup state) must not silently drop reads. nil ws +
// auto policy → true, matching the pre-demand-model behaviour.
func TestShouldReadNilWSIsPermissive(t *testing.T) {
	if !shouldRead("pod-1", "tick", nil, nil) {
		t.Fatal("nil ws + auto: want true (permissive)")
	}
}

// TestShouldReadAnyOrsAcrossClasses: ReadTick feeds tick / objects /
// debug, so the runner needs "any of these wanted?" gating. Empty room
// + one always-on class → true; all auto + all empty → false.
func TestShouldReadAnyOrsAcrossClasses(t *testing.T) {
	ws := &stubDemand{occupied: map[string]bool{}}
	classes := []string{"tick", "objects", "debug"}

	if shouldReadAny("pod-1", classes, nil, ws) {
		t.Fatal("all auto + empty: want false")
	}

	policies := []capture.Policy{
		{Instance: "pod-1", Class: "objects", Mode: capture.ModeAlways},
	}
	if !shouldReadAny("pod-1", classes, policies, ws) {
		t.Fatal("one class with always + empty: want true")
	}
}
