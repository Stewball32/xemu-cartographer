package halo2_test

// Live integration test: exercises the real Detect -> GameReader -> Read path
// against a running xemu Halo 2 instance. Inert unless H2_QMP_SOCK is set:
//
//	H2_QMP_SOCK=/run/user/1000/xemu-qmp-3way-h2-1.sock \
//	    go test ./internal/scraper/halo2 -run TestLiveH2 -v
//
// Confirms: (1) the scraper recognises the H2 title id via the registry,
// (2) low-GVA static pointers translate, (3) roster + biped state read live.

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	_ "github.com/Stewball32/xemu-cartographer/internal/scraper/halo2" // register H2
	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

func TestLiveH2(t *testing.T) {
	sock := os.Getenv("H2_QMP_SOCK")
	if sock == "" {
		t.Skip("set H2_QMP_SOCK to run the live H2 integration test")
	}

	inst := &xemu.Instance{Name: "h2-live", QMPSock: sock}
	// Mirror the manager: detection GVAs first so Detect can read the title id.
	if err := inst.Init(scraper.DetectionGVAs()); err != nil {
		t.Fatalf("Init(detection): %v", err)
	}
	defer inst.Close()

	reader, titleID, err := scraper.Detect(inst, "h2-live", "")
	if err != nil {
		t.Fatalf("Detect: %v (title 0x%08X)", err, titleID)
	}
	if titleID != 0x4D530064 {
		t.Fatalf("expected H2 title 0x4D530064, got 0x%08X", titleID)
	}
	t.Logf("Detect recognised title 0x%08X -> %q", titleID, reader.Title())

	// Re-init with detection + the H2 reader's low GVAs (as loop.go does).
	if err := inst.Init(append(scraper.DetectionGVAs(), reader.LowGVAs()...)); err != nil {
		t.Fatalf("Init(low GVAs): %v", err)
	}

	gs, tick, err := reader.ReadGameState()
	if err != nil {
		t.Fatalf("ReadGameState: %v", err)
	}
	t.Logf("ReadGameState -> state=%s tick=%d inputs=%v", gs, tick, reader.LastStateInputs())

	gd, err := reader.ReadGameData()
	if err != nil {
		t.Fatalf("ReadGameData: %v", err)
	}
	b, _ := json.MarshalIndent(gd, "", "  ")
	t.Logf("ReadGameData ->\n%s", b)

	tr, err := reader.ReadTick(gd.PowerItemSpawns, reader.NewTickState())
	if err != nil {
		t.Fatalf("ReadTick: %v", err)
	}
	tb, _ := json.MarshalIndent(tr.Payload, "", "  ")
	t.Logf("ReadTick payload ->\n%s", tb)

	if gs == scraper.GameStateInGame && len(tr.Payload.Players) == 0 {
		t.Fatalf("in_game but no players read")
	}
}
