// Package rating is the M18 rating + leaderboard core: the Elo update (18a/18b)
// and leaderboard ranking (18c). Pure math, no PB / no I/O, so it's fully
// unit-testable; the per-game-type recompute hook (18b), leaderboard pages
// (18c), and Discord commands (18d) are deferred (they need persistence + the
// live bot) but all sit on top of these functions.
//
// 18a algorithm choice — Elo, over Glicko-2 / TrueSkill, for v1:
//   - Simplest to reason about and test deterministically; zero-sum, so ratings
//     stay anchored around the default without inflation.
//   - Adequate for the 1v1 / two-team Halo matches this system records.
//   - Glicko-2 adds rating-deviation (uncertainty) bands — better for sparse
//     play, but more per-player state + harder to test. TrueSkill handles FFA /
//     arbitrary team sizes natively — the right call if/when FFA ratings matter
//     (factor graphs, heavier). FFA (N-way) rating is a documented follow-up;
//     v1 updates one 1v1 / two-team outcome.
//
// Per-game-type rating (18b) is a property of the caller: a player has a
// separate (rating, gametype) pair, and the caller feeds the matching pair of
// ratings into Update — the math here is game-type agnostic.
package rating

import "math"

const (
	// DefaultRating is the starting Elo for an unrated player.
	DefaultRating = 1500.0
	// DefaultK is the Elo K-factor (max single-game swing). 32 is the classic
	// chess value; tune per game type later if needed.
	DefaultK = 32.0
)

// Outcome scores from the "A" side's perspective.
const (
	WinA  = 1.0
	Draw  = 0.5
	LossA = 0.0
)

// Expected returns A's expected score against B under the logistic Elo curve —
// roughly A's win probability (counting a draw as half). Equal ratings give
// 0.5; Expected(a,b) + Expected(b,a) == 1.
func Expected(ratingA, ratingB float64) float64 {
	return 1.0 / (1.0 + math.Pow(10, (ratingB-ratingA)/400.0))
}

// Update applies one Elo result and returns the new (A, B) ratings. scoreA is
// WinA / Draw / LossA. The update is zero-sum: newA + newB == ratingA + ratingB,
// and an upset (the lower-rated side winning) moves ratings further than an
// expected result.
func Update(ratingA, ratingB, scoreA, k float64) (newA, newB float64) {
	delta := k * (scoreA - Expected(ratingA, ratingB))
	return ratingA + delta, ratingB - delta
}
