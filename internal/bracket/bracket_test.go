package bracket

import (
	"fmt"
	"sort"
	"testing"
)

func names(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("p%d", i+1)
	}
	return out
}

func TestSingleElim_PowerOfTwo(t *testing.T) {
	b := SingleElim(names(8))
	if b.Size != 8 || b.Rounds != 3 {
		t.Fatalf("size/rounds = %d/%d, want 8/3", b.Size, b.Rounds)
	}
	if len(b.FirstRound) != 4 {
		t.Fatalf("first round matches = %d, want 4", len(b.FirstRound))
	}
	// Standard seed order for 8: [1 8 4 5 2 7 3 6].
	want := []Match{{"p1", "p8"}, {"p4", "p5"}, {"p2", "p7"}, {"p3", "p6"}}
	for i, m := range b.FirstRound {
		if m != want[i] {
			t.Errorf("match %d = %v, want %v", i, m, want[i])
		}
	}
}

func TestSingleElim_FourSeeding(t *testing.T) {
	b := SingleElim(names(4))
	if b.Size != 4 || b.Rounds != 2 {
		t.Fatalf("size/rounds = %d/%d, want 4/2", b.Size, b.Rounds)
	}
	want := []Match{{"p1", "p4"}, {"p2", "p3"}}
	for i, m := range b.FirstRound {
		if m != want[i] {
			t.Errorf("match %d = %v, want %v", i, m, want[i])
		}
	}
}

func TestSingleElim_ByesForNonPowerOfTwo(t *testing.T) {
	b := SingleElim(names(5)) // size 8 → 3 byes (seeds 6,7,8 empty)
	if b.Size != 8 || b.Rounds != 3 {
		t.Fatalf("size/rounds = %d/%d, want 8/3", b.Size, b.Rounds)
	}
	byes, real := 0, 0
	for _, m := range b.FirstRound {
		if m.IsBye() {
			byes++
		} else {
			real++
		}
	}
	if byes != 3 || real != 1 {
		t.Errorf("byes/real = %d/%d, want 3/1", byes, real)
	}
	// Top 3 seeds get the byes; the one real match is the lowest seeds, p4 v p5.
	found := false
	for _, m := range b.FirstRound {
		if !m.IsBye() {
			if (m.A == "p4" && m.B == "p5") || (m.A == "p5" && m.B == "p4") {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected the one real match to be p4 v p5")
	}
}

func TestSingleElim_TinyCounts(t *testing.T) {
	if b := SingleElim(names(1)); b.Rounds != 0 {
		t.Errorf("1 participant rounds = %d, want 0", b.Rounds)
	}
	b := SingleElim(names(2))
	if b.Size != 2 || b.Rounds != 1 || len(b.FirstRound) != 1 {
		t.Fatalf("2 participants: size/rounds/matches = %d/%d/%d, want 2/1/1", b.Size, b.Rounds, len(b.FirstRound))
	}
	if b.FirstRound[0] != (Match{"p1", "p2"}) {
		t.Errorf("final = %v, want {p1 p2}", b.FirstRound[0])
	}
}

// collectPairs returns each match as a sorted "A|B" key for set comparison.
func collectPairs(rounds []Round) []string {
	var out []string
	for _, r := range rounds {
		for _, m := range r.Matches {
			a, b := m.A, m.B
			if a > b {
				a, b = b, a
			}
			out = append(out, a+"|"+b)
		}
	}
	sort.Strings(out)
	return out
}

func TestRoundRobin_Even(t *testing.T) {
	rounds := RoundRobin(names(4))
	if len(rounds) != 3 {
		t.Fatalf("rounds = %d, want 3", len(rounds))
	}
	pairs := collectPairs(rounds)
	// C(4,2) = 6 unique pairings, each exactly once.
	want := []string{"p1|p2", "p1|p3", "p1|p4", "p2|p3", "p2|p4", "p3|p4"}
	if len(pairs) != len(want) {
		t.Fatalf("pairings = %d, want %d (%v)", len(pairs), len(want), pairs)
	}
	for i := range want {
		if pairs[i] != want[i] {
			t.Errorf("pair %d = %s, want %s", i, pairs[i], want[i])
		}
	}
}

func TestRoundRobin_Odd(t *testing.T) {
	rounds := RoundRobin(names(3))
	if len(rounds) != 3 {
		t.Fatalf("rounds = %d, want 3 (odd → bye each round)", len(rounds))
	}
	pairs := collectPairs(rounds)
	want := []string{"p1|p2", "p1|p3", "p2|p3"} // C(3,2) = 3
	if len(pairs) != 3 {
		t.Fatalf("pairings = %d, want 3 (%v)", len(pairs), pairs)
	}
	for i := range want {
		if pairs[i] != want[i] {
			t.Errorf("pair %d = %s, want %s", i, pairs[i], want[i])
		}
	}
	// No round has more than one real match (the odd player byes each round).
	for _, r := range rounds {
		if len(r.Matches) != 1 {
			t.Errorf("round %d has %d matches, want 1", r.Number, len(r.Matches))
		}
	}
}

func TestRoundRobin_TooFew(t *testing.T) {
	if RoundRobin(names(1)) != nil {
		t.Error("RoundRobin with <2 participants should be nil")
	}
}
