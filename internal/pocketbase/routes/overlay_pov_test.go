package routes

import (
	"testing"

	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
	sc "github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// fakeInspect implements scraperiface.Inspect for the resolver test.
type fakeInspect struct {
	infos  []scraperiface.Info
	states map[string]scraperiface.InspectState
}

func (f fakeInspect) List() []scraperiface.Info { return f.infos }
func (f fakeInspect) Inspect(name string) (scraperiface.InspectState, bool) {
	s, ok := f.states[name]
	return s, ok
}

func b(v bool) *bool { return &v }

// A host "stream" with a System Link lobby of stream/BlueBox/RedBox, and a
// second idle instance whose own console is "stewball32" (mirrors live beta).
func fixture() fakeInspect {
	streamGD := &sc.GameData{
		Gametype:   "slayer",
		IsTeamGame: true,
		Machines: []sc.GameMachine{
			{Index: 0, Name: "stream", IsLocal: b(true)},
			{Index: 1, Name: "BlueBox", IsLocal: b(false)},
			{Index: 2, Name: "RedBox", IsLocal: b(false)},
		},
	}
	return fakeInspect{
		infos: []scraperiface.Info{
			{Name: "beta-stream", XboxName: "stream"},
			{Name: "beta-play", XboxName: "stewball32"},
		},
		states: map[string]scraperiface.InspectState{
			"beta-stream": {GameData: streamGD},
			"beta-play":   {GameData: nil}, // idle, no lobby
		},
	}
}

func TestResolveConsole(t *testing.T) {
	f := fixture()
	cases := []struct {
		console      string
		wantInstance string
		wantMachine  int
	}{
		{"RedBox", "beta-stream", 2},    // remote lobby peer → machine 2
		{"BlueBox", "beta-stream", 1},   // remote lobby peer → machine 1
		{"stream", "beta-stream", 0},    // host's own console is machine 0 in its lobby
		{"redbox", "beta-stream", 2},    // case-insensitive
		{"stewball32", "beta-play", -1}, // own xbox_name, no live lobby → whole instance
	}
	for _, c := range cases {
		inst, mi, _, _, ok := resolveConsole(f, c.console)
		if !ok {
			t.Errorf("%q: not resolved, want %s/%d", c.console, c.wantInstance, c.wantMachine)
			continue
		}
		if inst != c.wantInstance || mi != c.wantMachine {
			t.Errorf("%q resolved to %s/%d, want %s/%d", c.console, inst, mi, c.wantInstance, c.wantMachine)
		}
	}

	if _, _, _, _, ok := resolveConsole(f, "GreenBox"); ok {
		t.Error("unknown console should not resolve")
	}
}
