// Package bracket holds the M16b tournament-structure generators: turn a list
// of seeded participants + a format into the round structure. Pure
// combinatorics, no PB / no I/O, so it's fully unit-testable — the M16a schema,
// M16c bracket UI, and M16d tournament-aware series spawning are deferred (they
// need persistence + live updates to verify) but build on this.
package bracket

// Match is one pairing. B == "" is a bye: A advances automatically. For
// round-robin every Match is a real pairing.
type Match struct {
	A string
	B string
}

// IsBye reports whether the match is a bye (one side absent).
func (m Match) IsBye() bool { return m.A == "" || m.B == "" }

// Round is one round of a structure.
type Round struct {
	Number  int
	Matches []Match
}

// Bracket is a generated single-elimination structure. Only the first round can
// be paired concretely (later rounds depend on winners); Size + Rounds describe
// the full shape the M16c UI renders.
type Bracket struct {
	// Size is the bracket size — the next power of two ≥ participant count.
	Size int
	// Rounds is the total number of rounds (log2(Size)); 0 for ≤1 participant.
	Rounds int
	// FirstRound is the concrete round-1 pairing, with top seeds receiving
	// byes when the count isn't a power of two.
	FirstRound []Match
}

// SingleElim builds the single-elimination bracket over the seeded
// participants (participants[0] is the top seed). Standard bracket seeding
// keeps the top two seeds apart until the final; non-power-of-two counts give
// the highest seeds first-round byes.
func SingleElim(participants []string) Bracket {
	n := len(participants)
	if n <= 1 {
		size := n
		return Bracket{Size: size, Rounds: 0}
	}

	size := 1
	rounds := 0
	for size < n {
		size *= 2
		rounds++
	}

	order := seedOrder(size) // bracket slot order, 1-based seeds
	matches := make([]Match, 0, size/2)
	for i := 0; i < len(order); i += 2 {
		a := seedName(order[i], n, participants)
		b := seedName(order[i+1], n, participants)
		matches = append(matches, Match{A: a, B: b})
	}
	return Bracket{Size: size, Rounds: rounds, FirstRound: matches}
}

// seedName maps a 1-based seed to a participant, or "" when the seed exceeds
// the participant count (a bye slot).
func seedName(seed, n int, participants []string) string {
	if seed <= n {
		return participants[seed-1]
	}
	return ""
}

// seedOrder returns the standard bracket seeding slot order for a power-of-two
// size: pairs of adjacent entries are the round-1 matchups, arranged so seed 1
// and seed 2 can only meet in the final. e.g. size 4 → [1 4 2 3];
// size 8 → [1 8 4 5 2 7 3 6].
func seedOrder(size int) []int {
	seeds := []int{1}
	for len(seeds) < size {
		sum := len(seeds)*2 + 1
		next := make([]int, 0, len(seeds)*2)
		for _, s := range seeds {
			next = append(next, s, sum-s)
		}
		seeds = next
	}
	return seeds
}

// roundRobinBye is the sentinel participant inserted when the count is odd; the
// player paired against it sits that round out.
const roundRobinBye = ""

// RoundRobin builds a round-robin schedule where every participant plays every
// other exactly once, using the circle method. An odd count adds a bye each
// round. Returns N-1 rounds (even N) or N rounds (odd N).
func RoundRobin(participants []string) []Round {
	if len(participants) < 2 {
		return nil
	}

	ps := append([]string{}, participants...)
	if len(ps)%2 == 1 {
		ps = append(ps, roundRobinBye)
	}
	n := len(ps)
	roundsCount := n - 1

	rounds := make([]Round, 0, roundsCount)
	for r := 0; r < roundsCount; r++ {
		matches := make([]Match, 0, n/2)
		for i := 0; i < n/2; i++ {
			a, b := ps[i], ps[n-1-i]
			if a != roundRobinBye && b != roundRobinBye {
				matches = append(matches, Match{A: a, B: b})
			}
		}
		rounds = append(rounds, Round{Number: r + 1, Matches: matches})

		// Rotate all but the first entry clockwise (the circle method).
		last := ps[n-1]
		copy(ps[2:], ps[1:n-1])
		ps[1] = last
	}
	return rounds
}
