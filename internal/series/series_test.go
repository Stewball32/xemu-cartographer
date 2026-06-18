package series

import "testing"

func TestProgress(t *testing.T) {
	tests := []struct {
		name       string
		format     string
		targetN    int
		teamWins   []int
		wantDone   bool
		wantWinner int
		wantClinch int
		wantRemain int
		wantPlayed int
	}{
		// single
		{"single not started", FormatSingle, 0, []int{0, 0}, false, NoWinner, 1, 1, 0},
		{"single decided", FormatSingle, 0, []int{1, 0}, true, 0, 1, 0, 1},

		// best-of-5 → clinch 3
		{"bo5 in progress 2-1", FormatBestOfN, 5, []int{2, 1}, false, NoWinner, 3, 1, 3},
		{"bo5 clinched 3-1", FormatBestOfN, 5, []int{3, 1}, true, 0, 3, 0, 4},
		{"bo5 other team clinches", FormatBestOfN, 5, []int{1, 3}, true, 1, 3, 0, 4},
		{"bo3 → clinch 2", FormatBestOfN, 3, []int{2, 0}, true, 0, 2, 0, 2},

		// first-to-2
		{"first-to-2 in progress", FormatFirstToX, 2, []int{1, 0}, false, NoWinner, 2, 1, 1},
		{"first-to-2 decided", FormatFirstToX, 2, []int{0, 2}, true, 1, 2, 0, 2},

		// exact-n: play exactly N, winner = most wins
		{"exact-3 in progress", FormatExactN, 3, []int{1, 1}, false, NoWinner, 0, 1, 2},
		{"exact-3 done, winner", FormatExactN, 3, []int{2, 1}, true, 0, 0, 0, 3},
		{"exact-2 done, tie → no winner", FormatExactN, 2, []int{1, 1}, true, NoWinner, 0, 0, 2},

		// degenerate configs still terminate
		{"first-to-x zero target clamps to 1", FormatFirstToX, 0, []int{1, 0}, true, 0, 1, 0, 1},
		{"unknown format behaves like single", "weird", 5, []int{0, 1}, true, 1, 1, 0, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Progress(tc.format, tc.targetN, tc.teamWins)
			if got.Complete != tc.wantDone {
				t.Errorf("Complete = %v, want %v", got.Complete, tc.wantDone)
			}
			if got.WinnerTeam != tc.wantWinner {
				t.Errorf("WinnerTeam = %d, want %d", got.WinnerTeam, tc.wantWinner)
			}
			if got.Clinch != tc.wantClinch {
				t.Errorf("Clinch = %d, want %d", got.Clinch, tc.wantClinch)
			}
			if got.Remaining != tc.wantRemain {
				t.Errorf("Remaining = %d, want %d", got.Remaining, tc.wantRemain)
			}
			if got.GamesPlayed != tc.wantPlayed {
				t.Errorf("GamesPlayed = %d, want %d", got.GamesPlayed, tc.wantPlayed)
			}
		})
	}
}

func TestProgress_FourTeamFirstTo3(t *testing.T) {
	// More than two participants (round-robin-ish): first team to 3 wins.
	got := Progress(FormatFirstToX, 3, []int{1, 3, 2, 0})
	if !got.Complete || got.WinnerTeam != 1 {
		t.Errorf("got complete=%v winner=%d, want complete=true winner=1", got.Complete, got.WinnerTeam)
	}
}
