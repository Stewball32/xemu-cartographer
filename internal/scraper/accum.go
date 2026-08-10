package scraper

import "math"

// Match accumulator — the port of HaloCaster's extract_events stat layer
// (atlas/HaloCaster/HaloCE/halocaster.py:2291-2307 + get_empty_player_meta
// 2237-2255). CE's player_datum shots_fired/shots_hit fields read 0 live (never
// engine-maintained in this build), so HaloCaster COUNTED shots itself from
// per-tick weapon-ammo deltas — and likewise grenades, melees, damage, and
// powerup pickups. This is the same idea as the kill-chain's Prev* diffing, but
// self-contained: MatchAccum keeps its own per-player previous-tick state so it
// stays a pure, unit-testable value fed one TickResult at a time.
//
// It also derives the ACTIVITY LATCH used for dummy detection: a local seat is
// presumed a dummy until it shows real input — any input-action bit, any
// accumulated increment, a kill, or horizontal movement past a small threshold.
// Z is deliberately excluded from the movement test (an out-of-bounds dummy can
// FALL — physics moves it with zero input), and movement only accrues across
// alive→alive tick pairs (respawn teleports would otherwise trip the latch).
// Once latched Active, a player stays Active for the rest of the match.

// ActivityMoveThreshold is the cumulative horizontal displacement (world
// units; 1 wu ≈ 3 m) past which a player counts as "moving on purpose".
// Walking crosses it within a few hundred ms; jitter/physics drift does not.
const ActivityMoveThreshold = 0.25

// PlayerAccum is one player's accumulated per-match stats — the immutable
// snapshot handed to the wire/filter layers.
type PlayerAccum struct {
	ShotsFired        int32   `json:"shots_fired"`
	GrenadeThrows     int16   `json:"grenade_throws"`
	Melees            int16   `json:"melees"`
	DamageDealt       float32 `json:"damage_dealt"`    // to OTHERS (self-splash excluded)
	DamageReceived    float32 `json:"damage_received"` // includes self-inflicted
	CamoPickups       int16   `json:"camo_pickups"`
	OvershieldPickups int16   `json:"overshield_pickups"`
	BestKillStreak    uint16  `json:"best_kill_streak"`
	// Active is the dummy-detection latch: true once this player has shown any
	// real activity this match (sticky until the next match reset).
	Active bool `json:"active"`
}

// accumPrev is the per-player previous-tick state MatchAccum diffs against.
type accumPrev struct {
	seen        bool // first Observe for this player only baselines
	alive       bool
	x, y        float32
	cumMoveXY   float64
	frags       uint8
	plasmas     uint8
	meleeing    bool
	camo        bool
	overshield  bool
	kills       int16
	ammoBySlot  map[int]weaponAmmoPrev // key: weapon slot
	dmgTimes    [DamageTableSlots]uint32
	dmgBaseline bool
}

type weaponAmmoPrev struct {
	objectID uint32
	ammoMag  int16
	isEnergy bool
	charge   float32
	hasMag   bool
	hasCharg bool
}

// MatchAccum accumulates one match's per-player stats. Loop-goroutine only —
// callers publish Snapshot() copies for cross-goroutine readers. Create a
// fresh one at each match start (Ready→Live).
type MatchAccum struct {
	stats map[int]*PlayerAccum
	prev  map[int]*accumPrev
}

func NewMatchAccum() *MatchAccum {
	return &MatchAccum{
		stats: map[int]*PlayerAccum{},
		prev:  map[int]*accumPrev{},
	}
}

// Observe diffs one tick against the previous one, updating cumulative stats
// and the activity latch. The first observation of each player only baselines
// (HaloCaster diffed old-vs-new game_info the same way — no counts from the
// first frame).
func (a *MatchAccum) Observe(res TickResult) {
	internals := make(map[int]*InternalPlayerState, len(res.InternalPlayers))
	for i := range res.InternalPlayers {
		internals[res.InternalPlayers[i].Index] = &res.InternalPlayers[i]
	}

	for i := range res.Payload.Players {
		tp := &res.Payload.Players[i]
		ip := internals[tp.Index]
		st := a.stats[tp.Index]
		if st == nil {
			st = &PlayerAccum{}
			a.stats[tp.Index] = st
		}
		pv := a.prev[tp.Index]
		if pv == nil {
			pv = &accumPrev{ammoBySlot: map[int]weaponAmmoPrev{}}
			a.prev[tp.Index] = pv
		}

		activity := false

		// --- input-action bits: unambiguous human input, immune to physics ---
		if tp.IsShooting || tp.IsFiring || tp.IsMeleeing || tp.IsThrowingGrenade ||
			tp.IsJumping || tp.IsCrouching {
			activity = true
		}

		if pv.seen {
			// --- shots: magazine-ammo decrease; energy weapons count 1 per
			// charge decrease (halocaster.py:2299-2307) ---
			for wi := range tp.Weapons {
				w := &tp.Weapons[wi]
				prevW, ok := pv.ammoBySlot[w.Slot]
				if ok && prevW.objectID == w.ObjectID {
					if w.IsEnergy {
						if prevW.hasCharg && w.Charge != nil && *w.Charge < prevW.charge {
							st.ShotsFired++
							activity = true
						}
					} else if prevW.hasMag && w.AmmoMag != nil && *w.AmmoMag < prevW.ammoMag {
						st.ShotsFired += int32(prevW.ammoMag - *w.AmmoMag)
						activity = true
					}
				}
			}

			// --- grenade throws: nade-count decrease (pickups increase) ---
			if tp.Frags < pv.frags {
				st.GrenadeThrows += int16(pv.frags - tp.Frags)
				activity = true
			}
			if tp.Plasmas < pv.plasmas {
				st.GrenadeThrows += int16(pv.plasmas - tp.Plasmas)
				activity = true
			}

			// --- melee: rising edge ---
			if tp.IsMeleeing && !pv.meleeing {
				st.Melees++
			}

			// --- powerup pickups: rising edges ---
			if tp.HasCamo && !pv.camo {
				st.CamoPickups++
			}
			if tp.HasOvershield && !pv.overshield {
				st.OvershieldPickups++
			}

			// --- horizontal movement (alive→alive pairs only; Z excluded) ---
			if tp.Alive && pv.alive {
				dx, dy := float64(tp.X-pv.x), float64(tp.Y-pv.y)
				pv.cumMoveXY += math.Hypot(dx, dy)
				if pv.cumMoveXY > ActivityMoveThreshold {
					activity = true
				}
			}
		}

		if ip != nil {
			// --- best (peak) kill streak — the real SPREE stat ---
			if ip.KillStreak > st.BestKillStreak {
				st.BestKillStreak = ip.KillStreak
			}
			// A kill is activity; deaths/suicides are NOT (a dummy can be
			// killed, or fall to its death — both passive).
			if pv.seen && ip.Kills > pv.kills {
				activity = true
			}

			// --- damage: victim's damage table; new entry = slot time changed.
			// Received credits the victim (self-damage included); Dealt credits
			// the dealer player, excluding self-splash so DMG means "to others".
			for s := 0; s < DamageTableSlots; s++ {
				e := ip.DamageTable[s]
				if e.DamageTime == 0xFFFFFFFF {
					continue
				}
				if pv.dmgBaseline && e.DamageTime != pv.dmgTimes[s] {
					st.DamageReceived += e.Amount
					if e.DealerPlrHandle != 0xFFFFFFFF {
						dealer := int(e.DealerPlrHandle & 0xFFFF)
						if dealer != tp.Index {
							ds := a.stats[dealer]
							if ds == nil {
								ds = &PlayerAccum{}
								a.stats[dealer] = ds
							}
							ds.DamageDealt += e.Amount
						}
					}
				}
				pv.dmgTimes[s] = e.DamageTime
			}
			pv.dmgBaseline = true
			pv.kills = ip.Kills
		}

		if activity {
			st.Active = true
		}

		// --- rebaseline for the next tick ---
		pv.seen = true
		pv.alive = tp.Alive
		pv.x, pv.y = tp.X, tp.Y
		pv.frags, pv.plasmas = tp.Frags, tp.Plasmas
		pv.meleeing = tp.IsMeleeing
		pv.camo, pv.overshield = tp.HasCamo, tp.HasOvershield
		for wi := range tp.Weapons {
			w := &tp.Weapons[wi]
			p := weaponAmmoPrev{objectID: w.ObjectID, isEnergy: w.IsEnergy}
			if w.AmmoMag != nil {
				p.ammoMag, p.hasMag = *w.AmmoMag, true
			}
			if w.Charge != nil {
				p.charge, p.hasCharg = *w.Charge, true
			}
			pv.ammoBySlot[w.Slot] = p
		}
	}
}

// Snapshot returns a fresh copy of the per-player accumulated stats, safe to
// hand across goroutines (the caller may store/read it freely; Observe never
// mutates a returned snapshot).
func (a *MatchAccum) Snapshot() map[int]PlayerAccum {
	out := make(map[int]PlayerAccum, len(a.stats))
	for idx, st := range a.stats {
		out[idx] = *st
	}
	return out
}
