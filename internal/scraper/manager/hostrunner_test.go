package manager

import (
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/hostrunner"
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

func TestStateInputInt(t *testing.T) {
	si := scraper.StateInputs{
		"main_menu":       uint8(1),
		"game_connection": uint16(2),
		"wide":            uint32(3),
		"signed":          int32(-4),
		"str":             "nope",
	}
	cases := map[string]int{
		"main_menu":       1,
		"game_connection": 2,
		"wide":            3,
		"signed":          -4,
		"str":             0, // non-integer → 0
		"missing":         0, // absent → 0
	}
	for key, want := range cases {
		if got := stateInputInt(si, key); got != want {
			t.Errorf("stateInputInt(%q) = %d, want %d", key, got, want)
		}
	}
}

func TestDistinctTeams(t *testing.T) {
	// Roster present → distinct Team values.
	gd := scraper.GameData{Players: []scraper.GamePlayer{
		{Team: 0}, {Team: 1}, {Team: 0},
	}}
	if got := distinctTeams(gd); got != 2 {
		t.Errorf("distinctTeams(players 0,1,0) = %d, want 2", got)
	}
	// Empty roster → fall back to team-score slot count.
	gd = scraper.GameData{TeamScores: []scraper.TeamScore{{}, {}}}
	if got := distinctTeams(gd); got != 2 {
		t.Errorf("distinctTeams(2 team scores, no roster) = %d, want 2", got)
	}
	if got := distinctTeams(scraper.GameData{}); got != 0 {
		t.Errorf("distinctTeams(empty) = %d, want 0", got)
	}
}

// fakeReader is a minimal GameReader whose only meaningful method is
// LastStateInputs, so buildHostReadout's reader→observation wiring (main_menu /
// game_connection) is testable with no live container.
type fakeReader struct{ si scraper.StateInputs }

func (f *fakeReader) LowGVAs() []uint32                                 { return nil }
func (f *fakeReader) ReadGameState() (scraper.GameState, uint32, error) { return "", 0, nil }
func (f *fakeReader) LastStateInputs() scraper.StateInputs              { return f.si }
func (f *fakeReader) BuildScoreProbe() scraper.ScoreProbe               { return nil }
func (f *fakeReader) ReadGameData() (scraper.GameData, error)           { return scraper.GameData{}, nil }
func (f *fakeReader) ReadReadyState() (scraper.GameData, error)         { return scraper.GameData{}, nil }
func (f *fakeReader) ReadTick([]scraper.PowerItemSpawn, *scraper.TickState) (scraper.TickResult, error) {
	return scraper.TickResult{}, nil
}
func (f *fakeReader) DetectEvents(uint32, string, scraper.GameData, scraper.TickResult, *scraper.TickState) []scraper.Envelope {
	return nil
}
func (f *fakeReader) OnStateChange(prev, next scraper.GameState) error { return nil }
func (f *fakeReader) NewTickState() *scraper.TickState                 { return nil }
func (f *fakeReader) Title() string                                    { return "Halo: CE" }

// buildHostReadout projects the loop's reader state + GameData into the runner
// Observation. With game_connection=2 (hosting) and a readable system-link lobby
// (2 boxes, 2 teams) the classified screen is a lobby that is ready to start.
func TestBuildHostReadoutLobby(t *testing.T) {
	r := &runner{name: "pod1"}
	r.reader = &fakeReader{si: scraper.StateInputs{
		"main_menu":       uint8(0),
		"game_connection": uint16(2), // hosting
	}}
	r.gameData = scraper.GameData{
		Map:      "Blood Gulch",
		Gametype: "Team Slayer",
		Machines: []scraper.GameMachine{{Index: 0}, {Index: 1}},
		Players:  []scraper.GamePlayer{{Team: 0}, {Team: 1}},
	}
	ro := r.buildHostReadout(scraper.GameStatePreGame, 42)
	if !ro.Fresh || ro.Tick != 42 {
		t.Fatalf("readout freshness/tick wrong: %+v", ro)
	}
	if ro.Map != "Blood Gulch" || ro.Gametype != "Team Slayer" {
		t.Fatalf("map/gametype = %q/%q", ro.Map, ro.Gametype)
	}
	if ro.GameConnection != 2 || ro.MenuActive {
		t.Fatalf("game_connection/menu_active wrong: conn=%d menu=%v", ro.GameConnection, ro.MenuActive)
	}
	if ro.MachineCount != 2 || ro.TeamCount != 2 || ro.PlayerCount != 2 {
		t.Fatalf("counts wrong: machines=%d teams=%d players=%d", ro.MachineCount, ro.TeamCount, ro.PlayerCount)
	}
	obs := ro.Observation()
	if !obs.ReadyToStart() {
		t.Fatalf("2 boxes + 2 teams should be ReadyToStart: %+v", obs)
	}
	if s := hostrunner.Classify(obs); s != hostrunner.ScreenLobby {
		t.Fatalf("hosting + readable map/machines should classify as lobby, got %s", s)
	}
}
