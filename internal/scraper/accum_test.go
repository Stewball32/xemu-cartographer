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
