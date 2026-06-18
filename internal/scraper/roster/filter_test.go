package roster

import (
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

func boolPtr(b bool) *bool { return &b }

func names(players []scraper.GamePlayer) []string {
	out := make([]string, len(players))
	for i, p := range players {
		out[i] = p.Name
	}
	return out
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestFilterRoster(t *testing.T) {
	roster := []scraper.GamePlayer{
		{Name: "Stewie", IsLocal: boolPtr(false)},
		{Name: "HostDummy", IsLocal: boolPtr(true)},
		{Name: "BlueFox", IsLocal: boolPtr(false)},
		{Name: "BOT_neutral", IsLocal: nil},
	}

	tests := []struct {
		name string
		cfg  Config
		want []string
	}{
		{
			name: "no config is a no-op",
			cfg:  Config{},
			want: []string{"Stewie", "HostDummy", "BlueFox", "BOT_neutral"},
		},
		{
			name: "neutral host drops the local player",
			cfg:  Config{IsNeutralHost: true},
			want: []string{"Stewie", "BlueFox", "BOT_neutral"},
		},
		{
			name: "global allowlist drops by sanitized name",
			cfg:  Config{DummyGamertags: BuildDummySet([]string{"  BOT_NEUTRAL  "})},
			want: []string{"Stewie", "HostDummy", "BlueFox"},
		},
		{
			name: "both filters combine",
			cfg:  Config{IsNeutralHost: true, DummyGamertags: BuildDummySet([]string{"bot_neutral"})},
			want: []string{"Stewie", "BlueFox"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FilterRoster(roster, tc.cfg)
			if !eq(names(got), tc.want) {
				t.Errorf("FilterRoster() = %v, want %v", names(got), tc.want)
			}
		})
	}

	// Input is never mutated.
	_ = FilterRoster(roster, Config{IsNeutralHost: true})
	if len(roster) != 4 {
		t.Errorf("FilterRoster mutated its input: len=%d, want 4", len(roster))
	}
}

func TestFilterRoster_NilPreserved(t *testing.T) {
	if got := FilterRoster(nil, Config{IsNeutralHost: true}); got != nil {
		t.Errorf("FilterRoster(nil) = %v, want nil", got)
	}
}

func TestBuildDummySet(t *testing.T) {
	got := BuildDummySet([]string{"Alpha", "  beta ", "", "ALPHA"})
	if _, ok := got["alpha"]; !ok {
		t.Error("expected 'alpha' in set")
	}
	if _, ok := got["beta"]; !ok {
		t.Error("expected 'beta' in set")
	}
	if len(got) != 2 {
		t.Errorf("BuildDummySet len = %d, want 2 (dedup + drop empty)", len(got))
	}
	if BuildDummySet(nil) != nil {
		t.Error("BuildDummySet(nil) should be nil")
	}
}
