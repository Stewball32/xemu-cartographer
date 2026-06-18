// Package series holds the pure M14 series-format logic: given a series format
// + per-team win counts, decide whether the series is decided and who won. It's
// the load-bearing core of M14d (series-aware game-end wiring auto-ends a series
// when its format is satisfied) and feeds the M14c in-progress UI's "X–Y,
// best-of-5" standing line. Pure functions, no PB / no I/O — the wiring and UI
// that consume it are deferred (they need a live game to verify end to end).
package series

// Format constants mirror what the M14 series-setup UI writes to series.format.
// The M13 schema's SelectField currently only enumerates exact-n / first-to-x;
// M14a widens it to the set below, so these constants are the single source of
// truth the schema update + the wiring + the UI all reference.
const (
	// FormatSingle is a one-game series (a pickup match). targetN ignored.
	FormatSingle = "single"
	// FormatExactN plays exactly N games; the winner is whoever has the most
	// wins once N are played (ties possible → no winner).
	FormatExactN = "exact-n"
	// FormatBestOfN is decided when a team clinches a majority of N
	// (floor(N/2)+1), e.g. best-of-5 clinches at 3.
	FormatBestOfN = "best-of-n"
	// FormatFirstToX is decided when a team reaches X wins.
	FormatFirstToX = "first-to-x"
)

// NoWinner is the WinnerTeam value when the series isn't decided (or ended in a
// tie under exact-n).
const NoWinner = -1

// Standing is the computed state of a series from its per-team win counts.
type Standing struct {
	// Complete is true when the format's termination condition is met.
	Complete bool
	// WinnerTeam is the index (into the teamWins slice) of the winner, or
	// NoWinner when undecided / tied.
	WinnerTeam int
	// GamesPlayed is the total games recorded (sum of teamWins).
	GamesPlayed int
	// Clinch is the wins a single team needs to decide the series (the
	// majority threshold for best-of-n, X for first-to-x, 1 for single). 0
	// for exact-n, which is decided by game count, not a per-team threshold.
	Clinch int
	// Remaining is how many more games are guaranteed/possible before the
	// series must end: for threshold formats it's (Clinch - leader's wins)
	// floored at 0; for exact-n it's (targetN - GamesPlayed) floored at 0.
	Remaining int
}

// Progress computes the Standing for a series. teamWins[i] is the number of
// games team i has won so far (typically len 2, but any number of teams works —
// the winner is whichever team first satisfies the format). targetN is N for
// best-of-n / exact-n or X for first-to-x; it's ignored for single. Negative
// or zero targetN is clamped to 1 so a misconfigured series still terminates.
func Progress(format string, targetN int, teamWins []int) Standing {
	played := 0
	leader, leaderWins := NoWinner, -1
	tie := false
	for i, w := range teamWins {
		played += w
		if w > leaderWins {
			leader, leaderWins, tie = i, w, false
		} else if w == leaderWins {
			tie = true
		}
	}

	s := Standing{WinnerTeam: NoWinner, GamesPlayed: played}

	switch format {
	case FormatSingle:
		s.Clinch = 1
		if played >= 1 {
			s.Complete = true
			s.WinnerTeam = leader
		} else {
			s.Remaining = 1
		}

	case FormatFirstToX, FormatBestOfN:
		clinch := targetN
		if format == FormatBestOfN {
			clinch = targetN/2 + 1
		}
		if clinch < 1 {
			clinch = 1
		}
		s.Clinch = clinch
		if leaderWins >= clinch && !tie {
			s.Complete = true
			s.WinnerTeam = leader
		} else if leaderWins >= clinch && tie {
			// Two teams at the clinch line simultaneously can't happen in a
			// real best-of (you stop at the clinching game), but guard anyway.
			s.Complete = true
			s.WinnerTeam = NoWinner
		} else {
			s.Remaining = clinch - leaderWins
			if s.Remaining < 0 {
				s.Remaining = 0
			}
		}

	case FormatExactN:
		n := targetN
		if n < 1 {
			n = 1
		}
		if played >= n {
			s.Complete = true
			if !tie {
				s.WinnerTeam = leader
			}
		} else {
			s.Remaining = n - played
		}

	default:
		// Unknown format: treat like single so a bad config still terminates.
		s.Clinch = 1
		if played >= 1 {
			s.Complete = true
			s.WinnerTeam = leader
		} else {
			s.Remaining = 1
		}
	}

	return s
}
