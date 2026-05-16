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

// playerRefFromGame builds a v2 PlayerRef from a GamePlayer. Used by event
// detectors to denormalize roster identity (name, team, armor_color) onto
// emitted events so the event log alone is sufficient for analytics across
// later roster changes.
func playerRefFromGame(p scraper.GamePlayer) scraper.PlayerRef {
	return scraper.PlayerRef{
		Index:      p.Index,
		Name:       p.Name,
		Team:       p.Team,
		ArmorColor: p.ArmorColor,
	}
}

// playerRefByIndex returns a PlayerRef for idx, looking up identity in the
// game-data snapshot. When idx isn't in the roster (race during a join /
// leave), returns a partial ref with only Index set; the caller can check
// Name == "" to detect this.
func playerRefByIndex(snap scraper.GameData, idx int) scraper.PlayerRef {
	for _, p := range snap.Players {
		if p.Index == idx {
			return playerRefFromGame(p)
		}
	}
	return scraper.PlayerRef{Index: idx}
}

// vec3FromTickPlayer builds a v2 Vec3 position from a TickPlayer's flat
// X/Y/Z fields. Convenience wrapper so detectors don't repeat the
// conversion at every emit site.
func vec3FromTickPlayer(tp scraper.TickPlayer) scraper.Vec3 {
	return scraper.Vec3{X: tp.X, Y: tp.Y, Z: tp.Z}
}

// vec3Ptr returns &Vec3{x,y,z}. Useful for optional pos fields on event
// payloads (PlayerUpdateEvent.Pos, GameUpdateEvent.Pos).
func vec3Ptr(x, y, z float32) *scraper.Vec3 {
	v := scraper.Vec3{X: x, Y: y, Z: z}
	return &v
}

// playerRefPtr returns &p — convenience for optional PlayerRef fields
// (DeathEvent.Killer, DamageEvent.Dealer, GameUpdateEvent.Player).
func playerRefPtr(p scraper.PlayerRef) *scraper.PlayerRef {
	return &p
}

// vehicleRefPtr returns &v — convenience for optional VehicleRef fields.
func vehicleRefPtr(v scraper.VehicleRef) *scraper.VehicleRef {
	return &v
}

// itemRefPtr returns &i — convenience for optional ItemRef fields.
func itemRefPtr(i scraper.ItemRef) *scraper.ItemRef {
	return &i
}

// intPtr / uint16Ptr / uint8Ptr / int16Ptr / int32Ptr / uint32Ptr are
// generic-numeric pointer helpers for PlayerUpdateEvent / GameUpdateEvent
// fields that are pointer-typed so zero-valued numerics don't get dropped
// by `omitempty`.
func intPtr(v int) *int          { return &v }
func int16Ptr(v int16) *int16    { return &v }
func int32Ptr(v int32) *int32    { return &v }
func uint8Ptr(v uint8) *uint8    { return &v }
func uint16Ptr(v uint16) *uint16 { return &v }
func uint32Ptr(v uint32) *uint32 { return &v }
