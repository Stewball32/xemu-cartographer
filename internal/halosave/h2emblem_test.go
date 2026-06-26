package halosave

import (
	"os"
	"path/filepath"
	"testing"
)

// TestH2EmblemOffsetsFromRealProfiles locks in the CONFIRMED H2 emblem /
// appearance offset map (the 0x118 block) by decoding the two real captured
// profiles in testdata and asserting every named field. If the offsets or keys
// in H2ProfileFields drift, this fails — it is the regression guard behind the
// "confirmed" claim in h2profile.go / H2-EMBLEM-FORMAT.md.
//
// Expected values were read directly from the sample bytes and cross-checked
// against the in-game e_player_color / e_emblem_* enums:
//   587C…: purple/blue armor, pink/pink emblem, fg=thor(26), bg=quarter(22)
//   E4CADA… ("Stew"): cobalt/blue armor, white/white emblem, fg=jolly_roger(12),
//                     bg=horizontal_split2(3)
func TestH2EmblemOffsetsFromRealProfiles(t *testing.T) {
	cases := []struct {
		file string
		want map[string]int
	}{
		{"profile_587C76321326.bin", map[string]int{
			"armor_primary": 13, "armor_secondary": 11,
			"emblem_primary": 14, "emblem_secondary": 14,
			"character_type": 0, "emblem_foreground": 26,
			"emblem_background": 22, "emblem_flags": 0,
		}},
		{"profile_E4CADA6B1E65.bin", map[string]int{
			"armor_primary": 10, "armor_secondary": 11,
			"emblem_primary": 0, "emblem_secondary": 0,
			"character_type": 0, "emblem_foreground": 12,
			"emblem_background": 3, "emblem_flags": 0,
		}},
	}
	for _, c := range cases {
		b, err := os.ReadFile(filepath.Join("testdata", "h2", c.file))
		if err != nil {
			t.Fatalf("read %s: %v", c.file, err)
		}
		p, err := H2ProfileParse(b)
		if err != nil {
			t.Fatalf("parse %s: %v", c.file, err)
		}
		for k, want := range c.want {
			if got := p.Fields[k]; got != want {
				t.Errorf("%s: field %q = %d, want %d", c.file, k, got, want)
			}
		}
	}
}
