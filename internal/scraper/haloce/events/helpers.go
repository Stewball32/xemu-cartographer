package events

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// findTickPlayer returns the TickPlayer for index, or a zero-valued
// TickPlayer (with Index set) if not found.
func findTickPlayer(players []scraper.TickPlayer, index int) scraper.TickPlayer {
	for _, p := range players {
		if p.Index == index {
			return p
		}
	}
	return scraper.TickPlayer{Index: index}
}

// findInternal returns a pointer to the InternalPlayerState for index, or
// nil if not found.
func findInternal(players []scraper.InternalPlayerState, index int) *scraper.InternalPlayerState {
	for i := range players {
		if players[i].Index == index {
			return &players[i]
		}
	}
	return nil
}

// findKillerInDamageTable scans the victim's 4-slot damage table for entries
// within 5 ticks of "now" and returns the dealer player index of the most
// recent. Returns -1 when no entry attributes the kill. Used as a fallback
// when kill counters haven't ticked over yet for the killer.
func findKillerInDamageTable(ip scraper.InternalPlayerState, tick uint32) int {
	best := uint32(0)
	killerIdx := -1
	for _, e := range ip.DamageTable {
		if e.DamageTime == damageEmptySentinel {
			continue
		}
		if tick >= e.DamageTime && tick-e.DamageTime <= 5 {
			if e.DamageTime >= best {
				best = e.DamageTime
				killerIdx = int(e.DealerPlrHandle & handleIndexMask)
			}
		}
	}
	return killerIdx
}

// findRecentDealerInDamageTable returns the most recent damage dealer's
// player index for entries within 2 ticks of "now". Returns -1 when nothing
// matches. Used by the damage detector.
func findRecentDealerInDamageTable(ip scraper.InternalPlayerState, tick uint32) int {
	best := uint32(0)
	dealerIdx := -1
	for _, e := range ip.DamageTable {
		if e.DamageTime == damageEmptySentinel {
			continue
		}
		if tick >= e.DamageTime && tick-e.DamageTime <= 2 {
			if e.DamageTime >= best {
				best = e.DamageTime
				dealerIdx = int(e.DealerPlrHandle & handleIndexMask)
			}
		}
	}
	return dealerIdx
}

// findMeleeVictim looks up which player took damage from dealerIdx within
// the last 2 ticks. Returns -1 when no damage table entry attributes the
// melee. Used by the melee detector inside damage.go.
func findMeleeVictim(dealerIdx int, players []scraper.InternalPlayerState, tick uint32) int {
	for _, p := range players {
		if p.Index == dealerIdx {
			continue
		}
		for _, e := range p.DamageTable {
			if e.DamageTime == damageEmptySentinel {
				continue
			}
			if tick >= e.DamageTime && tick-e.DamageTime <= 2 {
				if int(e.DealerPlrHandle&handleIndexMask) == dealerIdx {
					return p.Index
				}
			}
		}
	}
	return -1
}

// gamePlayerByIndex builds a player-index → GamePlayer map from the
// game-data field in ctx. Several detectors need it (team_kill checks, roster
// diffs); each builds its own copy to keep Context lean.
func gamePlayerByIndex(snap scraper.GameData) map[int]scraper.GamePlayer {
	out := make(map[int]scraper.GamePlayer, len(snap.Players))
	for _, p := range snap.Players {
		out[p.Index] = p
	}
	return out
}
