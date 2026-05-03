package haloce

import (
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/haloce/events"
	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// TitleID is the Xbox title certificate ID for Halo: Combat Evolved (NTSC).
// Verified via XBE certificate read at GVA 0x00010000+0x118+0x0008 in the
// legacy Go port; matches the published Microsoft Game Studios product code.
//
// TODO(M7): re-confirm against PAL / debug builds if encountered.
const TitleID uint32 = 0x4D530004

// GametypeNames maps Halo: CE gametype IDs to human-readable strings.
//
// Origin: halocaster.py:1142-1150 + halocaster.py:2144-2167. The 12-14 entries
// are matching wildcards used by scenario item filters, not real gametypes —
// kept in the map so a stray scenario-item-derived value still renders.
var GametypeNames = map[uint32]string{
	0:  "none",
	1:  "ctf",
	2:  "slayer",
	3:  "oddball",
	4:  "king",
	5:  "race",
	6:  "terminator",
	7:  "stub",
	12: "all",
	13: "all_except_ctf",
	14: "all_except_ctf_race",
}

// Game implements scraper.GameReader for Halo: CE.
type Game struct {
	reader      *Reader
	xboxName    string
	nameScanned bool
}

// New creates a Halo CE GameReader for the given instance.
func New(inst *xemu.Instance, instanceName string) *Game {
	return &Game{reader: NewReader(inst, instanceName)}
}

func (g *Game) LowGVAs() []uint32 { return AllLowGVAs }

func (g *Game) ReadGameState() (scraper.GameState, uint32, error) {
	return g.reader.ReadGameState()
}

func (g *Game) LastStateInputs() scraper.StateInputs {
	return g.reader.LastStateInputs()
}

func (g *Game) BuildScoreProbe() scraper.ScoreProbe {
	return g.reader.BuildScoreProbe()
}

func (g *Game) ReadSnapshot() (scraper.SnapshotPayload, error) {
	return g.reader.ReadSnapshot()
}

func (g *Game) ReadLobby() (scraper.SnapshotPayload, error) {
	return g.reader.ReadLobby()
}

func (g *Game) ReadTick(spawns []scraper.PowerItemSpawn, state *scraper.TickState) (scraper.TickResult, error) {
	return g.reader.ReadTick(spawns, state)
}

func (g *Game) DetectEvents(tick uint32, instance string, snap scraper.SnapshotPayload, result scraper.TickResult, state *scraper.TickState) []scraper.Envelope {
	return events.Detect(tick, instance, snap, result, state)
}

func (g *Game) OnStateChange(prev, next scraper.GameState) error {
	return g.reader.OnStateChange(prev, next)
}

func (g *Game) NewTickState() *scraper.TickState {
	return scraper.NewTickState()
}

// Title returns the human-readable game title.
func (g *Game) Title() string { return "Halo: Combat Evolved" }

// XboxName returns the local xbox console name, scanning memory on first call
// and caching the result. The name doesn't change within a session.
func (g *Game) XboxName() string {
	if g.nameScanned {
		return g.xboxName
	}
	g.xboxName = ReadXboxName(g.reader.inst.Mem)
	g.nameScanned = true
	return g.xboxName
}

func init() {
	scraper.Register(TitleID, func(inst *xemu.Instance, instanceName string) scraper.GameReader {
		return New(inst, instanceName)
	})
}
