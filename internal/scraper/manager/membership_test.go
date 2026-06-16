package manager

import (
	"reflect"
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

func TestCollectIdentities(t *testing.T) {
	tests := []struct {
		name  string
		cache instanceCache
		want  []string
	}{
		{
			name:  "idle: only console name",
			cache: instanceCache{XboxName: "Console-A"},
			want:  []string{"console-a"},
		},
		{
			name:  "no data at all",
			cache: instanceCache{},
			want:  []string{},
		},
		{
			name: "console + machines + players, sanitized and sorted",
			cache: instanceCache{
				XboxName: "  Console-A  ",
				GameData: &scraper.GameData{
					Machines: []scraper.GameMachine{{Name: "Red-Box"}, {Name: "Blue-Box"}},
					Players:  []scraper.GamePlayer{{Name: "Stewie"}, {Name: "FoxTrot"}},
				},
			},
			// lowercased, trimmed, deduped, sorted
			want: []string{"blue-box", "console-a", "foxtrot", "red-box", "stewie"},
		},
		{
			name: "dedupes overlapping console / machine / player names",
			cache: instanceCache{
				XboxName: "Stewie",
				GameData: &scraper.GameData{
					Machines: []scraper.GameMachine{{Name: "STEWIE"}},
					Players:  []scraper.GamePlayer{{Name: "stewie"}, {Name: ""}},
				},
			},
			want: []string{"stewie"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := collectIdentities(&tc.cache)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("collectIdentities() = %v, want %v", got, tc.want)
			}
		})
	}
}
