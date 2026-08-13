package commands

import (
	"strings"
	"testing"
)

// TestBoxStatusEmbed covers the pure /box status embed builder for each
// resolution outcome (the live resolution itself is thin plumbing verified on a
// connected gateway).
func TestBoxStatusEmbed(t *testing.T) {
	cases := []struct {
		name      string
		container string
		res       resolveResult
		wantTitle string
		wantIn    string // substring expected in the description
	}{
		{"matched", "beta-play-abc", resolveMatched, "Your box", "beta-play-abc"},
		{"idle", "", resolveIdle, "Your box", "not in a live match"},
		{"not linked", "", resolveNotLinked, "Not linked", "isn't linked"},
		{"unavailable", "", resolveUnavailable, "Unavailable", "isn't running"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			emb := boxStatusEmbed(c.container, c.res)
			if emb.Title != c.wantTitle {
				t.Errorf("title = %q, want %q", emb.Title, c.wantTitle)
			}
			if !strings.Contains(emb.Description, c.wantIn) {
				t.Errorf("description %q missing %q", emb.Description, c.wantIn)
			}
		})
	}
}
