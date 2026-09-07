// External test package: the manager already imports rooms (runner.broadcast
// resolves its room names through RoomForInstanceClass), so an in-package
// test importing the manager would be an import cycle.
package rooms_test

import (
	"slices"
	"sort"
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/scraper/manager"
	"github.com/Stewball32/xemu-cartographer/internal/websocket/rooms"
)

// TestScraperClassRegistryMatchesRoomsTable pins the manager's class registry
// and the per-class room table to each other in BOTH directions: every class
// the manager announces in hello (except summary, which has its own
// cross-instance SummaryRoom) must be a per-instance room class, and every
// per-instance room class must be announced. The pre-M31 drift was the second
// direction — event / game_filtered / event_filtered were emittable rooms that
// hello and the capture sinks didn't know about — so a one-way check is not
// enough.
func TestScraperClassRegistryMatchesRoomsTable(t *testing.T) {
	m := manager.New(nil)
	defer m.Close()

	var announced []string
	for _, class := range m.BuildHelloPayload().Classes {
		if class == "summary" {
			continue
		}
		announced = append(announced, class)
	}
	sort.Strings(announced)

	table := rooms.ScraperClasses()
	if !slices.Equal(announced, table) {
		t.Fatalf("class registry and rooms table have drifted:\n  hello (minus summary) = %v\n  rooms table           = %v", announced, table)
	}
	for _, class := range table {
		if _, err := rooms.RoomForInstanceClass("alpha", class); err != nil {
			t.Errorf("class %q: %v", class, err)
		}
	}
}
