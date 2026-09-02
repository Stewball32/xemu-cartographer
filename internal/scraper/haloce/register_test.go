package haloce_test

import (
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	_ "github.com/Stewball32/xemu-cartographer/internal/scraper/haloce"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/offsets"
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

// TestGameSatisfiesLobbyInterfaces guards the exact runtime path that broke the
// map/gametype picker: the manager binds a GameReader from the REGISTERED FACTORY
// (a *Game wrapper, not a bare *Reader) and then type-asserts it to
// scraper.LobbyEnumerator (to fill the /api/play/options carousel cache) and
// scraper.LobbyCursorReader (to feed the host-runner's closed-loop card nav). The
// wrapper forwards ReadGameState etc. but once silently dropped these two
// optional methods — the asserts failed, the picker fell back to a type-in box,
// and select steps held forever. Assert the factory's concrete output implements
// both, so a dropped forward fails the test instead of shipping.
func TestGameSatisfiesLobbyInterfaces(t *testing.T) {
	f := scraper.Lookup(0x4D530004)
	if f == nil {
		t.Fatal("haloce not registered")
	}
	set, err := offsets.Baseline("haloce")
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}
	gr, err := f(nil, "test", set)
	if err != nil {
		t.Fatalf("factory: %v", err)
	}
	if _, ok := gr.(scraper.LobbyEnumerator); !ok {
		t.Error("factory GameReader does NOT implement scraper.LobbyEnumerator — /api/play/options would serve an empty picker (available=false → type-in box)")
	}
	if _, ok := gr.(scraper.LobbyCursorReader); !ok {
		t.Error("factory GameReader does NOT implement scraper.LobbyCursorReader — the host-runner's select-map/select-gametype steps would hold forever")
	}
}
