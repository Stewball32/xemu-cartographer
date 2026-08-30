package halo2

import (
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// TestGametypeName pins the induced e_game_engine mapping (2026-07-11: change
// only the gametype on a fixed map — ctf=1, slayer=2, oddball=3, king=4).
func TestGametypeName(t *testing.T) {
	cases := map[uint32]string{
		0: "", // boot / unset
		1: "ctf",
		2: "slayer",
		3: "oddball",
		4: "king",
		5: "", // future/unknown modes stay honestly blank
	}
	for in, want := range cases {
		if got := gametypeName(in); got != want {
			t.Errorf("gametypeName(%d) = %q, want %q", in, got, want)
		}
	}
}

// TestPhaseFromEnum pins the full-cycle-validated lifecycle enum
// (0→1→3→4→1 across menu → lobby → in-game → postgame → lobby) and that
// unvalidated values yield no verdict (the array inference decides).
func TestPhaseFromEnum(t *testing.T) {
	cases := []struct {
		in    uint32
		want  scraper.GameState
		known bool
	}{
		{0, scraper.GameStateMenu, true},
		{1, scraper.GameStatePreGame, true},
		{3, scraper.GameStateInGame, true},
		{4, scraper.GameStatePostGame, true},
		{2, scraper.GameStateMenu, false}, // never observed in validation
		{7, scraper.GameStateMenu, false},
	}
	for _, c := range cases {
		got, known := phaseFromEnum(c.in)
		if known != c.known || (known && got != c.want) {
			t.Errorf("phaseFromEnum(%d) = (%v,%v), want (%v,%v)", c.in, got, known, c.want, c.known)
		}
	}
}

// TestSlotDeltas pins the per-slot stat strides (kills u16 → 2, deaths u32 →
// 4, MAC entries → 6) and that all 16 player slots stay inside the base's 4K
// page — the single-page guarantee the low-read path relies on.
func TestSlotDeltas(t *testing.T) {
	if killsSlotDelta(3) != 6 || deathsSlotDelta(3) != 12 || macSlotDelta(3) != 18 {
		t.Fatalf("stride math wrong: %d %d %d",
			killsSlotDelta(3), deathsSlotDelta(3), macSlotDelta(3))
	}
	for slot := 0; slot < int(ConstH2PlayerMax); slot++ {
		kEnd := int64(AddrH2KillsPerPlayer) + killsSlotDelta(slot) + 2
		dEnd := int64(AddrH2DeathsPerPlayer) + deathsSlotDelta(slot) + 4
		if kEnd > (int64(AddrH2KillsPerPlayer)&^0xFFF)+0x1000 {
			t.Errorf("kills slot %d leaves the base page", slot)
		}
		if dEnd > (int64(AddrH2DeathsPerPlayer)&^0xFFF)+0x1000 {
			t.Errorf("deaths slot %d leaves the base page", slot)
		}
	}
	for i := 0; i < int(ConstH2NetMachineMax); i++ {
		end := int64(AddrH2NetMachineMacArray) + macSlotDelta(i) + 6
		if end > (int64(AddrH2NetMachineMacArray)&^0xFFF)+0x1000 {
			t.Errorf("MAC entry %d leaves the base page", i)
		}
	}
}

// TestMachineNameReadable pins the page guard: with the stock table at
// 0x54D91C, entries 0..9 read in-page (entry 9's span ends at 0x54DFD8) and
// entry 10 (base 0x54E024, past the page edge) is refused — a cross-page read
// through a single-page translation would return unrelated host memory.
func TestMachineNameReadable(t *testing.T) {
	for i := 0; i <= 9; i++ {
		if !machineNameReadable(AddrH2NetMachineTable, i, 64) {
			t.Errorf("entry %d should be readable", i)
		}
	}
	if machineNameReadable(AddrH2NetMachineTable, 10, 64) {
		t.Error("entry 10 leaves the page and must be refused")
	}
}

// TestScenarioBasename covers the pool-path decode: tag paths reduce to their
// basename; garbage, empty, and unterminated buffers are refused.
func TestScenarioBasename(t *testing.T) {
	ok := func(raw string, want string) {
		t.Helper()
		got, k := scenarioBasename(append([]byte(raw), 0))
		if !k || got != want {
			t.Errorf("scenarioBasename(%q) = (%q,%v), want (%q,true)", raw, got, k, want)
		}
	}
	ok(`scenarios\multi\zanzibar\zanzibar`, "zanzibar")
	ok(`scenarios/multi/midship/midship`, "midship")
	ok("turf", "turf")

	bad := [][]byte{
		{},                            // empty read
		{0},                           // empty string
		[]byte("no-nul-in-window"),    // unterminated
		append([]byte{0x01, 0x02}, 0), // non-printable
		append([]byte(`scenarios\multi\weird\`), 0x00), // trailing separator
	}
	for _, b := range bad {
		if got, k := scenarioBasename(b); k {
			t.Errorf("scenarioBasename(%v) accepted as %q", b, got)
		}
	}
}
