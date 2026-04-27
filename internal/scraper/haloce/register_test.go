package haloce_test

import (
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	_ "github.com/Stewball32/xemu-cartographer/internal/scraper/haloce"
)

// Sanity test: importing the haloce package should register the Halo CE title
// ID (0x4D530004) in the scraper registry. The blank import above triggers
// haloce.init(), which calls scraper.Register().
func TestHaloCERegistered(t *testing.T) {
	f := scraper.Lookup(0x4D530004)
	if f == nil {
		t.Fatal("haloce.init did not register Halo CE title ID 0x4D530004 with scraper.Lookup")
	}
}
