package embeds

import (
	"strings"
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/stats"
)

func TestUserStats(t *testing.T) {
	total := stats.Totals{Games: 3, Wins: 2, Losses: 1, Kills: 21, Deaths: 15, Assists: 2}
	byType := map[string]stats.Totals{
		"slayer": {Games: 2, Wins: 1, Kills: 18, Deaths: 13},
		"ctf":    {Games: 1, Wins: 1, Kills: 3, Deaths: 2},
	}
	e := UserStats("Stewie", total, byType)

	if !strings.Contains(e.Title, "Stewie") {
		t.Errorf("title = %q, want it to contain the gamertag", e.Title)
	}
	// Base fields (Games, W/L, K/D/A) + 2 per-gametype = 5.
	if len(e.Fields) != 5 {
		t.Fatalf("fields = %d, want 5 (3 base + 2 gametypes)", len(e.Fields))
	}
	if e.Fields[0].Name != "Games" || e.Fields[0].Value != "3" {
		t.Errorf("Games field = %+v", e.Fields[0])
	}
	// Per-gametype fields are sorted (ctf before slayer) for determinism.
	if e.Fields[3].Name != "ctf" || e.Fields[4].Name != "slayer" {
		t.Errorf("gametype fields not sorted: %q then %q", e.Fields[3].Name, e.Fields[4].Name)
	}
}

func TestUserStats_NoGames(t *testing.T) {
	e := UserStats("Nobody", stats.Totals{}, nil)
	if e.Description == "" {
		t.Error("empty stats should set a 'no games' description")
	}
}

func TestRecentGames(t *testing.T) {
	lines := []stats.PlayerGame{
		{Gametype: "slayer", Kills: 10, Deaths: 5, Won: true},
		{Gametype: "ctf", Kills: 3, Deaths: 8, Won: false},
		{Gametype: "slayer", Kills: 7, Deaths: 7, Won: true},
	}
	e := RecentGames("Stewie", lines, 2)
	if len(e.Fields) != 2 {
		t.Fatalf("recent fields = %d, want 2 (limit)", len(e.Fields))
	}
	if !strings.HasPrefix(e.Fields[0].Name, "W ") {
		t.Errorf("first recent should be a win: %q", e.Fields[0].Name)
	}
	if !strings.HasPrefix(e.Fields[1].Name, "L ") {
		t.Errorf("second recent should be a loss: %q", e.Fields[1].Name)
	}

	if RecentGames("x", nil, 5).Description == "" {
		t.Error("no games → description set")
	}
}

func TestGameResult(t *testing.T) {
	w := 0
	e := GameResult(GameResultInput{
		Map: "Blood Gulch", Gametype: "ctf", VariantName: "MLG CTF",
		ScoreSummary: "3 – 1", Category: "competitive", WinnerTeam: &w,
	})
	if !strings.Contains(e.Title, "Blood Gulch") {
		t.Errorf("title = %q, want the map", e.Title)
	}
	// VariantName overrides Gametype when present.
	var gt, score, winner, cat string
	for _, f := range e.Fields {
		switch f.Name {
		case "Gametype":
			gt = f.Value
		case "Score":
			score = f.Value
		case "Winner":
			winner = f.Value
		case "Category":
			cat = f.Value
		}
	}
	if gt != "MLG CTF" {
		t.Errorf("gametype = %q, want MLG CTF (variant override)", gt)
	}
	if score != "3 – 1" || cat != "competitive" || winner != "Team 0" {
		t.Errorf("fields: score=%q winner=%q cat=%q", score, winner, cat)
	}
}

func TestSeriesResult(t *testing.T) {
	w := 0
	e := SeriesResult("Grand Final", "best-of-n", []int{3, 1}, &w)
	var score, winner string
	for _, f := range e.Fields {
		if f.Name == "Score" {
			score = f.Value
		}
		if f.Name == "Winner" {
			winner = f.Value
		}
	}
	if score != "3 – 1" {
		t.Errorf("series score = %q, want '3 – 1'", score)
	}
	if winner != "Team 0" {
		t.Errorf("series winner = %q, want Team 0", winner)
	}
}
