package embeds

import (
	"strings"
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/rating"
)

func TestLeaderboard(t *testing.T) {
	if Leaderboard("slayer", nil).Description != "No rated players yet." {
		t.Error("empty leaderboard should say so")
	}
	entries := []rating.Entry{
		{Gamertag: "Stew", Rating: 1600, Games: 10},
		{Gamertag: "Fox", Rating: 1500, Games: 8},
	}
	e := Leaderboard("slayer", entries)
	if !strings.Contains(e.Title, "slayer") {
		t.Errorf("title = %q, want gametype", e.Title)
	}
	if !strings.Contains(e.Description, "1.") || !strings.Contains(e.Description, "Stew") {
		t.Errorf("description missing ranked entries: %q", e.Description)
	}
	// Stew (rank 1) should appear before Fox.
	if strings.Index(e.Description, "Stew") > strings.Index(e.Description, "Fox") {
		t.Error("leaderboard order wrong: Stew should precede Fox")
	}
}

func TestRankEmbed(t *testing.T) {
	if !strings.Contains(Rank("Stew", "slayer", 0, 0).Description, "Unrated") {
		t.Error("rank 0 should render as unrated")
	}
	e := Rank("Stew", "slayer", 3, 1575)
	if !strings.Contains(e.Description, "#3") || !strings.Contains(e.Description, "1575") {
		t.Errorf("rank embed = %q, want #3 + rating", e.Description)
	}
}
