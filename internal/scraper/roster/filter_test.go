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

// The unified activity rule (HideInactiveLocals): a LOCAL seat is presumed a
// dummy until the accumulator latches it Active; remotes are never affected;
// IsNeutralHost remains the hard override that hides locals even when Active.
func TestFilterRoster_ActivityRule(t *testing.T) {
	roster := []scraper.GamePlayer{
		{Index: 0, Name: "IdleLocal", IsLocal: boolPtr(true)},
		{Index: 1, Name: "MovingLocal", IsLocal: boolPtr(true)},
		{Index: 2, Name: "Remote", IsLocal: boolPtr(false)},
	}

	t.Run("inactive local hidden, active local shown, remote untouched", func(t *testing.T) {
		got := FilterRoster(roster, Config{
			HideInactiveLocals: true,
			ActiveLocals:       map[int]bool{1: true},
		})
		if !eq(names(got), []string{"MovingLocal", "Remote"}) {
			t.Fatalf("got %v", names(got))
		}
	})

	t.Run("nil ActiveLocals (pre-match) hides every local, keeps remotes", func(t *testing.T) {
		got := FilterRoster(roster, Config{HideInactiveLocals: true})
		if !eq(names(got), []string{"Remote"}) {
			t.Fatalf("got %v", names(got))
		}
	})

	t.Run("neutral-host override hides locals even when latched Active", func(t *testing.T) {
		got := FilterRoster(roster, Config{
			IsNeutralHost:      true,
			HideInactiveLocals: true,
			ActiveLocals:       map[int]bool{0: true, 1: true},
		})
		if !eq(names(got), []string{"Remote"}) {
			t.Fatalf("got %v", names(got))
		}
	})

	t.Run("rule off (zero config) passes everyone — debug surfaces unchanged", func(t *testing.T) {
		got := FilterRoster(roster, Config{})
		if !eq(names(got), []string{"IdleLocal", "MovingLocal", "Remote"}) {
			t.Fatalf("got %v", names(got))
		}
	})

	t.Run("allowlist still drops an active remote by name", func(t *testing.T) {
		got := FilterRoster(roster, Config{
			HideInactiveLocals: true,
			ActiveLocals:       map[int]bool{1: true},
			DummyGamertags:     BuildDummySet([]string{" REMOTE "}),
		})
		if !eq(names(got), []string{"MovingLocal"}) {
			t.Fatalf("got %v", names(got))
		}
	})
}
