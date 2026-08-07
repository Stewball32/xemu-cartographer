package haloce

import (
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/haloce/events"
	"github.com/Stewball32/xemu-cartographer/internal/scraper/offsets"
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
	reader *Reader
}

// Compile-time proof that the factory's concrete type (*Game — see init below)
// satisfies BOTH the base GameReader AND the OPTIONAL lobby interfaces the
// manager type-asserts on r.reader. These asserts are the ONLY reason the
// map/gametype picker + host-runner card navigation get live data; a *Game that
// forwards ReadGameState but silently drops EnumerateLobby/ReadLobbyCursor
// compiles fine yet fails those runtime asserts (the exact bug that left the
// picker as a type-in box). Keep these lines so the build breaks if a forward
// is removed. Also exercised by TestGameSatisfiesLobbyInterfaces.
var (
	_ scraper.GameReader        = (*Game)(nil)
	_ scraper.LobbyEnumerator   = (*Game)(nil)
	_ scraper.LobbyCursorReader = (*Game)(nil)
)

// New creates a Halo CE GameReader for the given instance, reading all address
// anchors through the given versioned offset set binding.
func New(inst *xemu.Instance, instanceName string, off Offsets) *Game {
	return &Game{reader: NewReader(inst, instanceName, off)}
}

func (g *Game) LowGVAs() []uint32 { return g.reader.off.AllLowGVAs() }

func (g *Game) ReadGameState() (scraper.GameState, uint32, error) {
	return g.reader.ReadGameState()
}

func (g *Game) LastStateInputs() scraper.StateInputs {
	return g.reader.LastStateInputs()
}

func (g *Game) BuildScoreProbe() scraper.ScoreProbe {
	return g.reader.BuildScoreProbe()
}

func (g *Game) ReadGameData() (scraper.GameData, error) {
	return g.reader.ReadGameData()
}

func (g *Game) ReadReadyState() (scraper.GameData, error) {
	return g.reader.ReadReadyState()
}

// EnumerateLobby forwards to the reader so *Game satisfies scraper.LobbyEnumerator.
// This is what the manager's enumerateLobby type-asserts on r.reader to populate
// the create-game map/gametype carousel cache that /api/play/options serves. The
// factory (init below) returns a *Game, NOT a bare *Reader, so WITHOUT this
// forward the assertion fails, the cache is never filled, and the player-picker
// degrades to free-text entry (available=false) even though the read works.
func (g *Game) EnumerateLobby() scraper.LobbyOptions { return g.reader.EnumerateLobby() }

// ReadLobbyCursor forwards to the reader so *Game satisfies scraper.LobbyCursorReader
// — the live SELECT MAP / SELECT GAMETYPE +0x4C cursor the host-runner's closed-loop
// card navigation reads (via fillLobbyCursor). WITHOUT this forward the assertion
// fails, MapCursorValid/GametypeCursorValid stay false, and the runner's select
// steps hold forever ("awaiting live cursor read") — so a submitted pick never
// drives the box.
func (g *Game) ReadLobbyCursor() scraper.LobbyCursor { return g.reader.ReadLobbyCursor() }

func (g *Game) ReadTick(spawns []scraper.PowerItemSpawn, state *scraper.TickState) (scraper.TickResult, error) {
	return g.reader.ReadTick(spawns, state)
}

func (g *Game) DetectEvents(tick uint32, instance string, snap scraper.GameData, result scraper.TickResult, state *scraper.TickState) []scraper.Envelope {
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

func init() {
	scraper.Register(TitleID, "haloce", func(inst *xemu.Instance, instanceName string, set *offsets.Set) (scraper.GameReader, error) {
		off, err := OffsetsFromSet(set)
		if err != nil {
			return nil, err
		}
		return New(inst, instanceName, off), nil
	})
}
