package rating

import "sort"

// Entry is one player's standing on a leaderboard (M18c). Built by the
// persistence layer from each player's per-game-type rating + game count.
type Entry struct {
	Gamertag string
	Rating   float64
	Games    int
}

// Rank returns the entries in leaderboard order — highest rating first, ties
// broken by more games played, then gamertag for a stable, deterministic order.
// The input slice is not mutated.
func Rank(entries []Entry) []Entry {
	out := append([]Entry{}, entries...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Rating != out[j].Rating {
			return out[i].Rating > out[j].Rating
		}
		if out[i].Games != out[j].Games {
			return out[i].Games > out[j].Games
		}
		return out[i].Gamertag < out[j].Gamertag
	})
	return out
}

// TopN returns the top n ranked entries (or all of them when n exceeds the
// count). n <= 0 returns an empty slice.
func TopN(entries []Entry, n int) []Entry {
	if n <= 0 {
		return []Entry{}
	}
	ranked := Rank(entries)
	if n < len(ranked) {
		return ranked[:n]
	}
	return ranked
}

// RankOf returns the 1-based leaderboard position of a gamertag, or 0 when the
// gamertag isn't present. Used by the M18d `/rank` Discord command.
func RankOf(entries []Entry, gamertag string) int {
	for i, e := range Rank(entries) {
		if e.Gamertag == gamertag {
			return i + 1
		}
	}
	return 0
}
