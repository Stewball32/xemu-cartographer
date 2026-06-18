// Package stats is the M15 aggregation layer over M13's games + game_players.
// The pure roll-up (this file) is split from the PocketBase query/projection
// (query.go) so the arithmetic — K/D, win rate, per-game-type splits, dummy
// exclusion — is fully unit-testable without a database. Per-user aggregation
// rolls a caller's gamertag lines together; per-team aggregation rolls a
// roster's lines; both are "feed the right PlayerGame slice into Roll".
package stats

// PlayerGame is one player's line in one game — a game_players row projected
// with the parent game's gametype / category / winner. The unit of aggregation.
type PlayerGame struct {
	Gamertag    string
	Gametype    string
	Category    string
	Team        int
	Kills       int
	Deaths      int
	Assists     int
	Score       int
	TimeAliveMs int64
	// Won is whether this player's team won the game. Computed upstream (the
	// projection compares the game's winner_team to the player's team).
	Won bool
	// IsDummy excludes the line from aggregates (M10d neutral-host dummy or
	// the M15d after-the-fact manual flag). Roll skips these.
	IsDummy bool
}

// Totals is a rolled-up stat line. Float ratios are derived via the methods so
// the struct stays a pure sum (mergeable, serializable).
type Totals struct {
	Games       int
	Wins        int
	Losses      int
	Kills       int
	Deaths      int
	Assists     int
	Score       int
	TimeAliveMs int64
}

// KD is kills ÷ deaths. With zero deaths it returns kills (i.e. deaths treated
// as 1) so a flawless line doesn't divide by zero.
func (t Totals) KD() float64 {
	if t.Deaths == 0 {
		return float64(t.Kills)
	}
	return float64(t.Kills) / float64(t.Deaths)
}

// KDA is (kills + assists) ÷ deaths, same zero-deaths handling as KD.
func (t Totals) KDA() float64 {
	if t.Deaths == 0 {
		return float64(t.Kills + t.Assists)
	}
	return float64(t.Kills+t.Assists) / float64(t.Deaths)
}

// WinRate is wins ÷ games in [0,1]; 0 when no games.
func (t Totals) WinRate() float64 {
	if t.Games == 0 {
		return 0
	}
	return float64(t.Wins) / float64(t.Games)
}

// Roll sums a set of PlayerGame lines into one Totals, skipping dummy lines. A
// line counts as a win or a loss (never both); Wins+Losses == Games.
func Roll(lines []PlayerGame) Totals {
	var t Totals
	for _, l := range lines {
		if l.IsDummy {
			continue
		}
		t.Games++
		if l.Won {
			t.Wins++
		} else {
			t.Losses++
		}
		t.Kills += l.Kills
		t.Deaths += l.Deaths
		t.Assists += l.Assists
		t.Score += l.Score
		t.TimeAliveMs += l.TimeAliveMs
	}
	return t
}

// RollByGametype groups lines by gametype before rolling each group, for the
// per-game-type split the stats UI shows. Dummy lines are skipped.
func RollByGametype(lines []PlayerGame) map[string]Totals {
	out := map[string]Totals{}
	groups := map[string][]PlayerGame{}
	for _, l := range lines {
		if l.IsDummy {
			continue
		}
		groups[l.Gametype] = append(groups[l.Gametype], l)
	}
	for gt, g := range groups {
		out[gt] = Roll(g)
	}
	return out
}

// FilterByGametype returns the lines whose Gametype equals gt.
func FilterByGametype(lines []PlayerGame, gt string) []PlayerGame {
	return filter(lines, func(l PlayerGame) bool { return l.Gametype == gt })
}

// FilterByCategory returns the lines whose Category equals cat.
func FilterByCategory(lines []PlayerGame, cat string) []PlayerGame {
	return filter(lines, func(l PlayerGame) bool { return l.Category == cat })
}

func filter(lines []PlayerGame, keep func(PlayerGame) bool) []PlayerGame {
	out := make([]PlayerGame, 0, len(lines))
	for _, l := range lines {
		if keep(l) {
			out = append(out, l)
		}
	}
	return out
}
