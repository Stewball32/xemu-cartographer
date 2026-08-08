package haloce

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// readGameGlobals reads the per-tick game_globals fields (at *AddrGameGlobalsPtr).
// Returns nil when the pointer is unset (e.g., during very early pregame before
// game_globals is allocated) — callers should treat nil as "not yet available".
//
// Source: OffGG* constants in offsets.go.
func (r *Reader) readGameGlobals() *scraper.TickGameGlobals {
	inst := r.inst
	mem := inst.Mem

	ggPtr, err := inst.DerefLowPtr(r.off.AddrGameGlobalsPtr)
	if err != nil || ggPtr < HighGVAThreshold {
		return nil
	}

	mapLoaded, _ := mem.ReadU8(ggPtr + OffGGMapLoaded)
	active, _ := mem.ReadU8(ggPtr + OffGGActive)
	doubleSpeed, _ := mem.ReadU8(ggPtr + OffGGPlayersAreDoubleSpeed)
	loadingInProgress, _ := mem.ReadU8(ggPtr + OffGGGameLoadingInProgress)
	precacheStatus, _ := mem.ReadF32(ggPtr + OffGGPrecacheMapStatus)
	difficulty, _ := mem.ReadU8(ggPtr + OffGGGameDifficultyLevel)
	storedRandom, _ := mem.ReadU32(ggPtr + OffGGStoredGlobalRandom)

	return &scraper.TickGameGlobals{
		MapLoaded:             mapLoaded,
		Active:                active,
		PlayersAreDoubleSpeed: doubleSpeed,
		GameLoadingInProgress: loadingInProgress,
		PrecacheMapStatus:     precacheStatus,
		GameDifficultyLevel:   difficulty,
		StoredGlobalRandom:    storedRandom,
	}
}

// readPregameSentinel reports whether game_globals+0x10 holds the pregame sentinel
// (0xDEADBEEF) — set the whole time the host is in pregame / on the SELECT MAP /
// SELECT GAMETYPE card screens (the read that pinned the "no effect" map bug).
// Cheap (1 deref + 1 u32); a raw diagnostic surface for the admin panel. False when
// game_globals isn't allocated yet.
func (r *Reader) readPregameSentinel() bool {
	inst := r.inst
	ggPtr, err := inst.DerefLowPtr(r.off.AddrGameGlobalsPtr)
	if err != nil || ggPtr < HighGVAThreshold {
		return false
	}
	v, err := inst.Mem.ReadU32(ggPtr + OffGGStoredGlobalRandom)
	if err != nil {
		return false
	}
	return v == PregameSentinel
}

// readLocalPlayerCount reads OffPGLocalPlayerCount from players_globals.
// Returns 0 when the pointer is unset.
func (r *Reader) readLocalPlayerCount() uint16 {
	inst := r.inst

	pgPtr, err := inst.DerefLowPtr(r.off.AddrPlayersGlobalsPtr)
	if err != nil || pgPtr < HighGVAThreshold {
		return 0
	}
	count, _ := inst.Mem.ReadU16(pgPtr + OffPGLocalPlayerCount)
	return count
}
