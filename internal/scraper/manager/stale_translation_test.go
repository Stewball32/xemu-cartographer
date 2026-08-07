package manager

import (
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// The silent-staleness detector. A scraper that attached mid-boot can cache low
// translations pointing at unbacked zero pages: every later read SUCCEEDS and
// returns 0, so no error ever surfaces and the error-driven refresh never fires.
// These pin the "all low globals zero is implausible" signal that catches it.
func TestLowReadsLookStale(t *testing.T) {
	zeros := scraper.StateInputs{
		"main_menu": uint8(0), "game_connection": uint16(0),
		"game_engine_globals_ptr": uint32(0), "game_time_globals_ptr": uint32(0),
		"game_can_score": uint32(0),
	}
	cases := []struct {
		name string
		si   scraper.StateInputs
		want bool
	}{
		{"all zero → stale (the live beta symptom)", zeros, true},
		{"at a real menu main_menu is 1", withKey(zeros, "main_menu", uint8(1)), false},
		{"hosting → game_connection 2", withKey(zeros, "game_connection", uint16(2)), false},
		{"in a game → engine ptr non-nil", withKey(zeros, "game_engine_globals_ptr", uint32(0x80061000)), false},
		{"game_can_score set", withKey(zeros, "game_can_score", uint32(1)), false},
		{"no read yet → not stale", scraper.StateInputs{}, false},
		// A plugin that doesn't report these keys opts out rather than being
		// guessed at — otherwise every non-CE title would look "stale".
		{"plugin omits the keys → opts out", scraper.StateInputs{"something_else": uint8(0)}, false},
	}
	for _, c := range cases {
		r := &runner{name: "pod1", reader: &fakeReader{si: c.si}}
		if got := r.lowReadsLookStale(); got != c.want {
			t.Errorf("%s: lowReadsLookStale() = %v, want %v", c.name, got, c.want)
		}
	}

	// No reader bound (Idle) must never claim staleness.
	if (&runner{name: "pod1"}).lowReadsLookStale() {
		t.Error("nil reader should not report stale")
	}
}

func withKey(base scraper.StateInputs, k string, v any) scraper.StateInputs {
	out := scraper.StateInputs{}
	for key, val := range base {
		out[key] = val
	}
	out[k] = v
	return out
}

// The stale gate backs off: this condition can persist forever, and each refresh
// costs a QMP round trip per low GVA, so it must not become a hot loop.
func TestBackoffGate(t *testing.T) {
	g := newBackoffGate(2, 8)

	fireAfter := func(n int) bool {
		for i := 0; i < n-1; i++ {
			if g.fail() {
				return false // fired too early
			}
		}
		return g.fail()
	}

	if !fireAfter(2) {
		t.Fatal("should fire on the 2nd failure")
	}
	if !fireAfter(4) {
		t.Fatal("after backoff should fire on the 4th")
	}
	if !fireAfter(8) {
		t.Fatal("after backoff should fire on the 8th")
	}
	if !fireAfter(8) {
		t.Fatal("backoff should CAP at max (8), not keep doubling")
	}

	// A good read restores the original cadence.
	g.ok()
	if !fireAfter(2) {
		t.Fatal("ok() should restore the base threshold")
	}
}
