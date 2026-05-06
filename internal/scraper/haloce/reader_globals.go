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

	ggPtr, err := inst.DerefLowPtr(AddrGameGlobalsPtr)
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

// readLocalPlayerCount reads OffPGLocalPlayerCount from players_globals.
// Returns 0 when the pointer is unset.
func (r *Reader) readLocalPlayerCount() uint16 {
	inst := r.inst

	pgPtr, err := inst.DerefLowPtr(AddrPlayersGlobalsPtr)
	if err != nil || pgPtr < HighGVAThreshold {
		return 0
	}
	count, _ := inst.Mem.ReadU16(pgPtr + OffPGLocalPlayerCount)
	return count
}
