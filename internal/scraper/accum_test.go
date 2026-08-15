package scraper

import "testing"

// Helpers to build minimal tick results.

func i16p(v int16) *int16     { return &v }
func f32p(v float32) *float32 { return &v }

func tick(players ...TickPlayer) TickResult {
	internals := make([]InternalPlayerState, 0, len(players))
	for _, p := range players {
		internals = append(internals, InternalPlayerState{Index: p.Index})
	}
	return TickResult{Payload: TickPayload{Players: players}, InternalPlayers: internals}
}

func alivePlayer(idx int) TickPlayer {
	return TickPlayer{Index: idx, Alive: true}
}

func TestAccum_ShotsFromMagazineDelta(t *testing.T) {
	a := NewMatchAccum()
	p := alivePlayer(0)
	p.Weapons = []WeaponInfo{{Slot: 0, ObjectID: 42, AmmoMag: i16p(12)}}
	a.Observe(tick(p)) // baseline

	p.Weapons = []WeaponInfo{{Slot: 0, ObjectID: 42, AmmoMag: i16p(9)}}
	a.Observe(tick(p))
	got := a.Snapshot()[0]
	if got.ShotsFired != 3 {
		t.Fatalf("ShotsFired = %d, want 3", got.ShotsFired)
	}
	if !got.Active {
		t.Fatal("firing must latch Active")
	}

	// Reload (mag increases) must NOT count.
	p.Weapons = []WeaponInfo{{Slot: 0, ObjectID: 42, AmmoMag: i16p(12)}}
	a.Observe(tick(p))
	if got := a.Snapshot()[0]; got.ShotsFired != 3 {
		t.Fatalf("reload counted as shots: %d", got.ShotsFired)
	}

	// Weapon swap (new ObjectID, lower ammo) must NOT count.
	p.Weapons = []WeaponInfo{{Slot: 0, ObjectID: 99, AmmoMag: i16p(2)}}
	a.Observe(tick(p))
	if got := a.Snapshot()[0]; got.ShotsFired != 3 {
		t.Fatalf("weapon swap counted as shots: %d", got.ShotsFired)
	}
}

func TestAccum_EnergyWeaponCountsOnePerChargeDrop(t *testing.T) {
	a := NewMatchAccum()
	p := alivePlayer(0)
	p.Weapons = []WeaponInfo{{Slot: 0, ObjectID: 7, IsEnergy: true, Charge: f32p(1.0)}}
	a.Observe(tick(p))

	p.Weapons = []WeaponInfo{{Slot: 0, ObjectID: 7, IsEnergy: true, Charge: f32p(0.94)}}
	a.Observe(tick(p))
	if got := a.Snapshot()[0]; got.ShotsFired != 1 {
		t.Fatalf("energy ShotsFired = %d, want 1 (one per decrease)", got.ShotsFired)
	}
}

func TestAccum_GrenadeThrowsFromCountDecrease(t *testing.T) {
	a := NewMatchAccum()
	p := alivePlayer(0)
	p.Frags, p.Plasmas = 2, 1
	a.Observe(tick(p))

	p.Frags = 1 // threw a frag
	a.Observe(tick(p))
	// Pickup (increase) must not count.
	p.Frags = 4
	a.Observe(tick(p))
	p.Plasmas = 0 // threw a plasma
	a.Observe(tick(p))

	got := a.Snapshot()[0]
	if got.GrenadeThrows != 2 {
		t.Fatalf("GrenadeThrows = %d, want 2", got.GrenadeThrows)
	}
}

func TestAccum_MeleeAndPickupRisingEdges(t *testing.T) {
	a := NewMatchAccum()
	p := alivePlayer(0)
	a.Observe(tick(p))

	p.IsMeleeing, p.HasCamo, p.HasOvershield = true, true, true
	a.Observe(tick(p))
	// Held states must not re-count.
	a.Observe(tick(p))
	got := a.Snapshot()[0]
	if got.Melees != 1 || got.CamoPickups != 1 || got.OvershieldPickups != 1 {
		t.Fatalf("rising edges: melee=%d camo=%d os=%d, want 1/1/1", got.Melees, got.CamoPickups, got.OvershieldPickups)
	}
}

func TestAccum_DamageTableDedupeAndAttribution(t *testing.T) {
	a := NewMatchAccum()
	victim := alivePlayer(1)
	res := tick(victim)
	// Baseline with an existing (stale) entry — must NOT count.
	res.InternalPlayers[0].DamageTable[0] = DamageEntry{DamageTime: 100, Amount: 30, DealerPlrHandle: 0x0000}
	a.Observe(res)

	// New damage event from player 0 (slot time advanced).
	res2 := tick(victim)
	res2.InternalPlayers[0].DamageTable[0] = DamageEntry{DamageTime: 160, Amount: 25, DealerPlrHandle: 0x0000}
	a.Observe(res2)
	// Same entry unchanged next tick — deduped.
	a.Observe(res2)

	snap := a.Snapshot()
	if snap[1].DamageReceived != 25 {
		t.Fatalf("victim DamageReceived = %v, want 25", snap[1].DamageReceived)
	}
	if snap[0].DamageDealt != 25 {
		t.Fatalf("dealer DamageDealt = %v, want 25", snap[0].DamageDealt)
	}
	if snap[1].Active {
		t.Fatal("taking damage must NOT latch the victim Active")
	}
}

func TestAccum_SelfDamageExcludedFromDealt(t *testing.T) {
	a := NewMatchAccum()
	p := alivePlayer(0)
	res := tick(p)
	a.Observe(res)
	res2 := tick(p)
	res2.InternalPlayers[0].DamageTable[0] = DamageEntry{DamageTime: 50, Amount: 40, DealerPlrHandle: 0x0000} // dealer index 0 == victim
	a.Observe(res2)
	snap := a.Snapshot()
	if snap[0].DamageDealt != 0 {
		t.Fatalf("self-splash counted as DamageDealt: %v", snap[0].DamageDealt)
	}
	if snap[0].DamageReceived != 40 {
		t.Fatalf("self-damage must count as Received: %v", snap[0].DamageReceived)
	}
}

func TestAccum_BestKillStreakIsPeak(t *testing.T) {
	a := NewMatchAccum()
	p := alivePlayer(0)
	for _, ks := range []uint16{1, 3, 5, 0, 2} { // dies at 5, rebuilds to 2
		res := tick(p)
		res.InternalPlayers[0].KillStreak = ks
		a.Observe(res)
	}
	if got := a.Snapshot()[0]; got.BestKillStreak != 5 {
		t.Fatalf("BestKillStreak = %d, want 5 (peak, not current)", got.BestKillStreak)
	}
}

func TestAccum_ActivityLatch(t *testing.T) {
	t.Run("input action bit latches immediately", func(t *testing.T) {
		a := NewMatchAccum()
		p := alivePlayer(0)
		p.IsJumping = true
		a.Observe(tick(p))
		if !a.Snapshot()[0].Active {
			t.Fatal("jump input must latch Active on first tick")
		}
	})

	t.Run("horizontal walk past threshold latches", func(t *testing.T) {
		a := NewMatchAccum()
		p := alivePlayer(0)
		a.Observe(tick(p)) // baseline at (0,0)
		for i := 0; i < 10; i++ {
			p.X += 0.04 // 10 × 0.04 = 0.4 wu > 0.25
			a.Observe(tick(p))
		}
		if !a.Snapshot()[0].Active {
			t.Fatal("walking 0.4 wu must latch Active")
		}
	})

	t.Run("falling dummy (Z-only movement) never latches", func(t *testing.T) {
		a := NewMatchAccum()
		p := alivePlayer(0)
		a.Observe(tick(p))
		for i := 0; i < 100; i++ {
			p.Z -= 1.0 // falling out of bounds forever
			a.Observe(tick(p))
		}
		if a.Snapshot()[0].Active {
			t.Fatal("Z-only movement (falling) must not latch Active")
		}
	})

	t.Run("respawn teleport does not latch", func(t *testing.T) {
		a := NewMatchAccum()
		p := alivePlayer(0)
		a.Observe(tick(p))
		p.Alive = false // dies
		a.Observe(tick(p))
		p.Alive = true // respawns across the map
		p.X, p.Y = 50, 50
		a.Observe(tick(p))
		if a.Snapshot()[0].Active {
			t.Fatal("respawn position jump must not latch Active")
		}
	})

	t.Run("kill latches, death does not", func(t *testing.T) {
		a := NewMatchAccum()
		p := alivePlayer(0)
		res := tick(p)
		a.Observe(res)
		res2 := tick(p)
		res2.InternalPlayers[0].Kills = 1
		a.Observe(res2)
		if !a.Snapshot()[0].Active {
			t.Fatal("a kill must latch Active")
		}
	})

	t.Run("latch is sticky", func(t *testing.T) {
		a := NewMatchAccum()
		p := alivePlayer(0)
		p.IsShooting = true
		a.Observe(tick(p))
		p.IsShooting = false
		for i := 0; i < 5; i++ {
			a.Observe(tick(p))
		}
		if !a.Snapshot()[0].Active {
			t.Fatal("Active must stay latched after activity stops")
		}
	})
}

// --- CE accuracy spec (halo-offset-mapper, 2026-08-15) -----------------------

// The damage table's Amount is the ACCUMULATED total from that dealer, not the
// size of the latest hit. The spec measured 31 pistol rounds climbing
// 25 → 50 → 75 → … → 800; the accumulator must report 800, not the sum of every
// intermediate reading (which is what adding the whole Amount per change did).
func TestAccum_DamageCountsGrowthNotRunningTotal(t *testing.T) {
	a := NewMatchAccum()
	victim := alivePlayer(1)

	base := tick(victim)
	base.InternalPlayers[0].DamageTable[0] = DamageEntry{DamageTime: 100, Amount: 0, DealerPlrHandle: 0x0000}
	a.Observe(base)

	// 32 rounds of exactly 25.0, cumulative, one engine tick apart.
	for n := 1; n <= 32; n++ {
		res := tick(victim)
		res.InternalPlayers[0].DamageTable[0] = DamageEntry{
			DamageTime:      uint32(100 + n),
			Amount:          float32(25 * n),
			DealerPlrHandle: 0x0000,
		}
		a.Observe(res)
	}

	snap := a.Snapshot()
	if snap[1].DamageReceived != 800 {
		t.Fatalf("victim DamageReceived = %v, want 800 (the final cumulative total)", snap[1].DamageReceived)
	}
	if snap[0].DamageDealt != 800 {
		t.Fatalf("dealer DamageDealt = %v, want 800", snap[0].DamageDealt)
	}
}

// Two rounds landing inside one engine tick advance Amount without advancing
// DamageTime — keying off the amount (not the timestamp) still counts both.
func TestAccum_DamageCountsGrowthWithinOneEngineTick(t *testing.T) {
	a := NewMatchAccum()
	victim := alivePlayer(1)

	base := tick(victim)
	base.InternalPlayers[0].DamageTable[0] = DamageEntry{DamageTime: 100, Amount: 10, DealerPlrHandle: 0x0000}
	a.Observe(base)

	same := tick(victim)
	same.InternalPlayers[0].DamageTable[0] = DamageEntry{DamageTime: 100, Amount: 30, DealerPlrHandle: 0x0000}
	a.Observe(same)

	if got := a.Snapshot()[1].DamageReceived; got != 20 {
		t.Fatalf("DamageReceived = %v, want 20 (growth within one tick)", got)
	}
}

// The 4 slots recycle between attackers: a slot handed to a new dealer starts a
// fresh running total, so its whole Amount is new damage.
func TestAccum_DamageSlotRecycledToNewDealerCountsWhole(t *testing.T) {
	a := NewMatchAccum()
	victim := alivePlayer(2)

	base := tick(victim)
	base.InternalPlayers[0].DamageTable[0] = DamageEntry{DamageTime: 100, Amount: 50, DealerPlrHandle: 0x0000}
	a.Observe(base)

	// Player 0 keeps hitting: 50 → 75 is 25 of new damage.
	res := tick(victim)
	res.InternalPlayers[0].DamageTable[0] = DamageEntry{DamageTime: 130, Amount: 75, DealerPlrHandle: 0x0000}
	a.Observe(res)

	// Slot now belongs to player 1, whose running total starts at 40.
	res2 := tick(victim)
	res2.InternalPlayers[0].DamageTable[0] = DamageEntry{DamageTime: 160, Amount: 40, DealerPlrHandle: 0x0001}
	a.Observe(res2)

	snap := a.Snapshot()
	if snap[0].DamageDealt != 25 {
		t.Fatalf("player 0 DamageDealt = %v, want 25 (growth only)", snap[0].DamageDealt)
	}
	if snap[1].DamageDealt != 40 {
		t.Fatalf("player 1 DamageDealt = %v, want 40 (fresh slot counts whole)", snap[1].DamageDealt)
	}
	if snap[2].DamageReceived != 65 {
		t.Fatalf("victim DamageReceived = %v, want 65", snap[2].DamageReceived)
	}
}

// An emptied slot drops its baseline, so reusing it later counts from zero
// rather than diffing against a stale total.
func TestAccum_DamageEmptiedSlotResetsBaseline(t *testing.T) {
	a := NewMatchAccum()
	victim := alivePlayer(1)

	base := tick(victim)
	base.InternalPlayers[0].DamageTable[0] = DamageEntry{DamageTime: 100, Amount: 90, DealerPlrHandle: 0x0000}
	a.Observe(base)

	empty := tick(victim) // all slots default to 0xFFFFFFFF = empty
	a.Observe(empty)

	reuse := tick(victim)
	reuse.InternalPlayers[0].DamageTable[0] = DamageEntry{DamageTime: 200, Amount: 30, DealerPlrHandle: 0x0000}
	a.Observe(reuse)

	if got := a.Snapshot()[0].DamageDealt; got != 30 {
		t.Fatalf("DamageDealt = %v, want 30 (reused slot counts whole, not 30-90)", got)
	}
}

// Shots come off magazine + reserve, so a reload is invisible and a sample that
// straddles a full dump-and-reload still counts every round.
func TestAccum_ShotsFromMagazinePlusReserve(t *testing.T) {
	a := NewMatchAccum()
	p := alivePlayer(0)
	// AR: 60 in the magazine, 240 in reserve.
	p.Weapons = []WeaponInfo{{Slot: 0, ObjectID: 42, AmmoMag: i16p(60), AmmoPack: i16p(240)}}
	a.Observe(tick(p))

	// Fire 10: 50 + 240 = 290.
	p.Weapons = []WeaponInfo{{Slot: 0, ObjectID: 42, AmmoMag: i16p(50), AmmoPack: i16p(240)}}
	a.Observe(tick(p))
	if got := a.Snapshot()[0].ShotsFired; got != 10 {
		t.Fatalf("ShotsFired = %d, want 10", got)
	}

	// Pure reload — 60 in the mag again, reserve pays for it. Total unchanged.
	p.Weapons = []WeaponInfo{{Slot: 0, ObjectID: 42, AmmoMag: i16p(60), AmmoPack: i16p(230)}}
	a.Observe(tick(p))
	if got := a.Snapshot()[0].ShotsFired; got != 10 {
		t.Fatalf("reload counted as shots: %d, want 10", got)
	}

	// The case magazine-only counting lost: one sample straddles a whole
	// magazine dump AND the reload, so the magazine reads 60 on both sides.
	p.Weapons = []WeaponInfo{{Slot: 0, ObjectID: 42, AmmoMag: i16p(60), AmmoPack: i16p(170)}}
	a.Observe(tick(p))
	if got := a.Snapshot()[0].ShotsFired; got != 70 {
		t.Fatalf("ShotsFired = %d, want 70 (60 rounds across a reload must not vanish)", got)
	}

	// Ammo pickup raises the total — never counts as shots.
	p.Weapons = []WeaponInfo{{Slot: 0, ObjectID: 42, AmmoMag: i16p(60), AmmoPack: i16p(240)}}
	a.Observe(tick(p))
	if got := a.Snapshot()[0].ShotsFired; got != 70 {
		t.Fatalf("pickup counted as shots: %d, want 70", got)
	}
}
