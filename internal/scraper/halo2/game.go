package halo2

import (
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// Game implements scraper.GameReader for Halo 2.
type Game struct {
	reader *Reader
}

// New creates a Halo 2 GameReader for the given instance.
func New(inst *xemu.Instance, instanceName string) *Game {
	return &Game{reader: NewReader(inst, instanceName)}
}

func (g *Game) LowGVAs() []uint32 { return AllLowGVAs }

func (g *Game) ReadGameState() (scraper.GameState, uint32, error) {
	return g.reader.ReadGameState()
}

func (g *Game) LastStateInputs() scraper.StateInputs { return g.reader.LastStateInputs() }

func (g *Game) BuildScoreProbe() scraper.ScoreProbe { return g.reader.BuildScoreProbe() }

func (g *Game) ReadGameData() (scraper.GameData, error) { return g.reader.ReadGameData() }

func (g *Game) ReadReadyState() (scraper.GameData, error) { return g.reader.ReadReadyState() }

func (g *Game) ReadTick(spawns []scraper.PowerItemSpawn, state *scraper.TickState) (scraper.TickResult, error) {
	return g.reader.ReadTick(spawns, state)
}

func (g *Game) DetectEvents(tick uint32, instance string, snap scraper.GameData, result scraper.TickResult, state *scraper.TickState) []scraper.Envelope {
	return g.reader.DetectEvents(tick, instance, snap, result, state)
}

func (g *Game) OnStateChange(prev, next scraper.GameState) error {
	return g.reader.OnStateChange(prev, next)
}

func (g *Game) NewTickState() *scraper.TickState { return scraper.NewTickState() }

// Title returns the human-readable game title.
func (g *Game) Title() string { return "Halo 2" }

func init() {
	scraper.Register(TitleID, func(inst *xemu.Instance, instanceName string) scraper.GameReader {
		return New(inst, instanceName)
	})
}

// ---------------------------------------------------------------------------
// Name tables (preserved from atlas/xemu-cartographer-legacy per M20; the
// gametype/damage tables are unused until their source offsets are re-derived).
// ---------------------------------------------------------------------------

// scenarioIDNames maps a loaded scenario tag id (tag_header+0xC) to the map's
// internal name. Runtime-verified entries only; extend as ids are confirmed.
var scenarioIDNames = map[uint32]string{
	0xE1780003: "turf", // verified live 2026-07-01 (splitscreen)
}

// mapDisplayNames converts internal scenario names to display names.
// Only "turf" is runtime-verified here; the rest are from the legacy reader.
var mapDisplayNames = map[string]string{
	"beavercreek":    "Beaver Creek",
	"burial_mounds":  "Burial Mounds",
	"coagulation":    "Coagulation",
	"colossus":       "Colossus",
	"cyclotron":      "Ivory Tower",
	"foundation":     "Foundation",
	"headlong":       "Headlong",
	"lockout":        "Lockout",
	"midship":        "Midship",
	"waterworks":     "Waterworks",
	"zanzibar":       "Zanzibar",
	"ascension":      "Ascension",
	"deltatap":       "Sanctuary",
	"dune":           "Relic",
	"elongation":     "Elongation",
	"gemini":         "Gemini",
	"triplicate":     "Terminal",
	"turf":           "Turf",
	"containment":    "Containment",
	"warlock":        "Warlock",
	"street_sweeper": "District",
	"needle":         "Uplift",
	"backwash":       "Backwash",
}

// GametypeNames maps Halo 2 gametype IDs to human-readable strings.
// UNVERIFIED source offset (legacy GRG gametype global stale) — table retained
// for when gametype detection is re-derived (M20).
var GametypeNames = map[uint8]string{
	0: "none", 1: "ctf", 2: "slayer", 3: "oddball",
	4: "koth", 7: "juggernaut", 8: "territories", 9: "assault",
}
