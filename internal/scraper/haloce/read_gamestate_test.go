package haloce

import (
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/hostrunner"
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// The runner's front-end-menu detection depends on ReadGameState landing
// main_menu + game_connection in LastStateInputs whenever they are readable —
// even when the game-engine reads (which sit before/around them) fail because
// the box is at the menu and that region isn't resident. These tests pin that
// contract: a failed game read must NOT swallow the readable menu fields, and
// only a failed main_menu read (a broken translation) surfaces as an error so
// the ready loop knows to re-translate.

// fakeMenuInstance builds a reader over an in-memory RAM image where the menu
// fields read successfully (main_menu=1, game_connection=0) but the game-engine
// addresses are mapped past the end of the image, so their reads fail.
func fakeMenuInstance(t *testing.T, mainMenuReadable bool) (*Reader, func()) {
	t.Helper()
	off := BaselineOffsets()

	// 16-byte RAM image: main_menu (u8) at 0, game_connection (u16) at 4.
	ram := make([]byte, 16)
	ram[0] = 1 // main_menu = 1 (at the menu)
	// game_connection stays 0 (ConnMenu).

	beyond := int64(len(ram) + 0x1000) // any offset past EOF → pread short-read
	mainMenuOff := int64(0)
	if !mainMenuReadable {
		mainMenuOff = beyond // simulate a stale/broken translation of main_menu
	}
	lowHVAs := map[uint32]int64{
		off.AddrMainMenuActive:       mainMenuOff,
		off.AddrGameConnection:       4,
		off.AddrGameEngineGlobalsPtr: beyond, // game-.data not resident at the menu
		off.AddrGameTimeGlobalsPtr:   beyond,
		off.AddrGameCanScore:         beyond,
	}
	inst, cleanup, err := xemu.NewTestInstance("test", ram, lowHVAs)
	if err != nil {
		t.Fatalf("new test instance: %v", err)
	}
	return NewReader(inst, "test", off), cleanup
}

// A game-engine read failing at the menu must not abort before the readable
// menu fields land — ReadGameState returns nil and the runner classifies the
// main menu from main_menu + game_connection alone.
func TestReadGameStateMenuSurvivesGameReadFailure(t *testing.T) {
	r, cleanup := fakeMenuInstance(t, true)
	defer cleanup()

	state, _, err := r.ReadGameState()
	if err != nil {
		t.Fatalf("ReadGameState returned error despite readable menu fields: %v", err)
	}
	si := r.LastStateInputs()
	if got := si["main_menu"]; got != uint8(1) {
		t.Errorf("state_inputs[main_menu] = %v (%T), want uint8(1)", got, got)
	}
	if got := si["game_connection"]; got != uint16(0) {
		t.Errorf("state_inputs[game_connection] = %v (%T), want uint16(0)", got, got)
	}
	if state != scraper.GameStateMenu {
		t.Errorf("game state = %q, want menu", state)
	}

	// End-to-end: the readout the manager builds from these inputs must classify
	// the main menu (mirrors buildHostReadout: MenuActive = main_menu!=0,
	// Connection = game_connection), which is what fires nav-system-link.
	obs := hostrunner.Observation{
		Fresh:      true,
		MenuActive: stInt(si, "main_menu") != 0,
		Connection: hostrunner.Connection(stInt(si, "game_connection")),
	}
	if got := hostrunner.Classify(obs); got != hostrunner.ScreenMainMenu {
		t.Errorf("Classify = %v, want main_menu", got)
	}
}

// A failed main_menu read is a broken low translation, not a benign
// game-region miss — ReadGameState must surface it as an error so the ready
// loop re-translates, while still having populated state_inputs.
func TestReadGameStateErrorsWhenMenuFieldUnreadable(t *testing.T) {
	r, cleanup := fakeMenuInstance(t, false)
	defer cleanup()

	_, _, err := r.ReadGameState()
	if err == nil {
		t.Fatal("ReadGameState returned nil when the main_menu read failed; the ready loop would never refresh")
	}
	// state_inputs is still written (defaults) so a subsequent successful read
	// recovers without special-casing.
	if _, ok := r.LastStateInputs()["main_menu"]; !ok {
		t.Error("state_inputs missing main_menu key after a failed read")
	}
}

// stInt mirrors manager.stateInputInt for the small subset the menu readout
// needs, avoiding an import of the manager package (which imports haloce).
func stInt(si map[string]any, key string) int {
	switch n := si[key].(type) {
	case uint8:
		return int(n)
	case uint16:
		return int(n)
	case uint32:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}
