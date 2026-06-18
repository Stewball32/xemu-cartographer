package rating

import (
	"math"
	"testing"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestExpected(t *testing.T) {
	if !approx(Expected(1500, 1500), 0.5) {
		t.Errorf("Expected(equal) = %v, want 0.5", Expected(1500, 1500))
	}
	if Expected(1600, 1500) <= 0.5 {
		t.Error("higher-rated player should have >0.5 expected score")
	}
	// Symmetry: Expected(a,b) + Expected(b,a) == 1.
	if !approx(Expected(1600, 1500)+Expected(1500, 1600), 1.0) {
		t.Error("Expected is not symmetric")
	}
}

func TestUpdate_EqualWin(t *testing.T) {
	a, b := Update(1500, 1500, WinA, DefaultK)
	// E=0.5, delta = 32*(1-0.5) = 16.
	if !approx(a, 1516) || !approx(b, 1484) {
		t.Errorf("Update equal win = (%v, %v), want (1516, 1484)", a, b)
	}
	// Zero-sum.
	if !approx(a+b, 3000) {
		t.Errorf("not zero-sum: a+b = %v, want 3000", a+b)
	}
}

func TestUpdate_Draw(t *testing.T) {
	a, b := Update(1500, 1500, Draw, DefaultK)
	if !approx(a, 1500) || !approx(b, 1500) {
		t.Errorf("equal draw should not move ratings; got (%v, %v)", a, b)
	}
}

func TestUpdate_UpsetMovesMore(t *testing.T) {
	// Underdog (1400) beats favorite (1600).
	upsetA, _ := Update(1400, 1600, WinA, DefaultK)
	upsetGain := upsetA - 1400
	// Expected result (1600 beats 1400) — favorite gains little.
	favA, _ := Update(1600, 1400, WinA, DefaultK)
	favGain := favA - 1600
	if upsetGain <= favGain {
		t.Errorf("upset gain (%v) should exceed expected-result gain (%v)", upsetGain, favGain)
	}
	// Sanity: upset gain is the large chunk of K we expect (~24.3).
	if upsetGain < 20 || upsetGain > 28 {
		t.Errorf("upset gain = %v, want ~24.3", upsetGain)
	}
}

func TestRank(t *testing.T) {
	entries := []Entry{
		{Gamertag: "low", Rating: 1400, Games: 10},
		{Gamertag: "high", Rating: 1700, Games: 5},
		{Gamertag: "midA", Rating: 1500, Games: 3},
		{Gamertag: "midB", Rating: 1500, Games: 8}, // same rating, more games → ranks above midA
	}
	ranked := Rank(entries)
	wantOrder := []string{"high", "midB", "midA", "low"}
	for i, w := range wantOrder {
		if ranked[i].Gamertag != w {
			t.Errorf("rank %d = %s, want %s", i, ranked[i].Gamertag, w)
		}
	}
	// Input not mutated.
	if entries[0].Gamertag != "low" {
		t.Error("Rank mutated its input")
	}
}

func TestTopNAndRankOf(t *testing.T) {
	entries := []Entry{
		{Gamertag: "a", Rating: 1600},
		{Gamertag: "b", Rating: 1500},
		{Gamertag: "c", Rating: 1400},
	}
	top2 := TopN(entries, 2)
	if len(top2) != 2 || top2[0].Gamertag != "a" || top2[1].Gamertag != "b" {
		t.Errorf("TopN(2) = %v, want [a b]", top2)
	}
	if TopN(entries, 0) == nil || len(TopN(entries, 0)) != 0 {
		t.Error("TopN(0) should be an empty slice")
	}
	if len(TopN(entries, 99)) != 3 {
		t.Error("TopN beyond count should return all")
	}
	if r := RankOf(entries, "b"); r != 2 {
		t.Errorf("RankOf(b) = %d, want 2", r)
	}
	if r := RankOf(entries, "missing"); r != 0 {
		t.Errorf("RankOf(missing) = %d, want 0", r)
	}
}
