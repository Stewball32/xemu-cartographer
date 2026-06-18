package stats

import (
	"math"
	"testing"
)

func sampleLines() []PlayerGame {
	return []PlayerGame{
		{Gamertag: "Stew", Gametype: "slayer", Category: "casual", Team: 0, Kills: 10, Deaths: 5, Assists: 2, Score: 25, Won: true, TimeAliveMs: 300000},
		{Gamertag: "Stew", Gametype: "slayer", Category: "casual", Team: 1, Kills: 8, Deaths: 8, Score: 18, Won: false, TimeAliveMs: 250000},
		{Gamertag: "Stew", Gametype: "ctf", Category: "competitive", Team: 0, Kills: 3, Deaths: 2, Score: 1, Won: true, TimeAliveMs: 400000},
		{Gamertag: "Stew", Gametype: "slayer", Kills: 99, Deaths: 0, Won: true, IsDummy: true}, // excluded
	}
}

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestRoll(t *testing.T) {
	tot := Roll(sampleLines())
	if tot.Games != 3 {
		t.Errorf("Games = %d, want 3 (dummy excluded)", tot.Games)
	}
	if tot.Wins != 2 || tot.Losses != 1 {
		t.Errorf("W/L = %d/%d, want 2/1", tot.Wins, tot.Losses)
	}
	if tot.Kills != 21 || tot.Deaths != 15 || tot.Assists != 2 {
		t.Errorf("K/D/A = %d/%d/%d, want 21/15/2", tot.Kills, tot.Deaths, tot.Assists)
	}
	if tot.Score != 44 {
		t.Errorf("Score = %d, want 44", tot.Score)
	}
	if tot.TimeAliveMs != 950000 {
		t.Errorf("TimeAliveMs = %d, want 950000", tot.TimeAliveMs)
	}
	if !approx(tot.KD(), 21.0/15.0) {
		t.Errorf("KD = %v, want %v", tot.KD(), 21.0/15.0)
	}
	if !approx(tot.WinRate(), 2.0/3.0) {
		t.Errorf("WinRate = %v, want %v", tot.WinRate(), 2.0/3.0)
	}
}

func TestTotals_ZeroDeaths(t *testing.T) {
	tot := Roll([]PlayerGame{{Kills: 5, Assists: 1, Deaths: 0, Won: true}})
	if !approx(tot.KD(), 5) {
		t.Errorf("KD with zero deaths = %v, want 5", tot.KD())
	}
	if !approx(tot.KDA(), 6) {
		t.Errorf("KDA with zero deaths = %v, want 6", tot.KDA())
	}
}

func TestTotals_EmptyWinRate(t *testing.T) {
	if wr := (Totals{}).WinRate(); wr != 0 {
		t.Errorf("empty WinRate = %v, want 0", wr)
	}
}

func TestRollByGametype(t *testing.T) {
	byGt := RollByGametype(sampleLines())
	if byGt["slayer"].Games != 2 {
		t.Errorf("slayer games = %d, want 2 (dummy excluded)", byGt["slayer"].Games)
	}
	if byGt["ctf"].Games != 1 {
		t.Errorf("ctf games = %d, want 1", byGt["ctf"].Games)
	}
	if _, ok := byGt[""]; ok {
		t.Error("the dummy line (gametype \"\") leaked into the split")
	}
}

func TestFilters(t *testing.T) {
	comp := FilterByCategory(sampleLines(), "competitive")
	if len(comp) != 1 || comp[0].Gametype != "ctf" {
		t.Errorf("FilterByCategory(competitive) = %d lines, want 1 ctf", len(comp))
	}
	slayer := FilterByGametype(sampleLines(), "slayer")
	if len(slayer) != 3 { // includes the dummy line; Roll excludes it later
		t.Errorf("FilterByGametype(slayer) = %d lines, want 3", len(slayer))
	}
}
