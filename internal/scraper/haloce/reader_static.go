package haloce

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// readPlayerSpawns walks the scenario's player-spawn array and returns one
// StaticPlayerSpawn per entry. Reads at snapshot time only (scenario data is
// map-static).
//
// Source: OffScenarioPlayerSpawn* + OffPlayerSpawn* constants.
func (r *Reader) readPlayerSpawns() []scraper.StaticPlayerSpawn {
	inst := r.inst
	mem := inst.Mem

	scenarioBase, err := inst.DerefLowPtr(AddrGlobalScenarioPtr)
	if err != nil || scenarioBase < HighGVAThreshold {
		return nil
	}

	count, _ := mem.ReadS32(scenarioBase + OffScenarioPlayerSpawnCount)
	if count <= 0 {
		return nil
	}
	firstAddr, _ := mem.ReadU32(scenarioBase + OffScenarioPlayerSpawnFirst)
	if firstAddr < HighGVAThreshold {
		return nil
	}

	out := make([]scraper.StaticPlayerSpawn, 0, count)
	for i := int32(0); i < count; i++ {
		base := firstAddr + uint32(i)*uint32(ScenarioPlayerSpawnStride)
		x, _ := mem.ReadF32(base + OffPlayerSpawnX)
		y, _ := mem.ReadF32(base + OffPlayerSpawnY)
		z, _ := mem.ReadF32(base + OffPlayerSpawnZ)
		facing, _ := mem.ReadF32(base + OffPlayerSpawnFacing)
		team, _ := mem.ReadU8(base + OffPlayerSpawnTeamIndex)
		bsp, _ := mem.ReadU8(base + OffPlayerSpawnBspIndex)
		unk0, _ := mem.ReadU16(base + OffPlayerSpawnUnk0)
		gt0, _ := mem.ReadU8(base + OffPlayerSpawnGametype0)
		gt1, _ := mem.ReadU8(base + OffPlayerSpawnGametype1)
		gt2, _ := mem.ReadU8(base + OffPlayerSpawnGametype2)
		gt3, _ := mem.ReadU8(base + OffPlayerSpawnGametype3)

		out = append(out, scraper.StaticPlayerSpawn{
			Index:     int(i),
			X:         x,
			Y:         y,
			Z:         z,
			Facing:    facing,
			TeamIndex: team,
			BspIndex:  bsp,
			Unk0:      unk0,
			Gametype0: gt0,
			Gametype1: gt1,
			Gametype2: gt2,
			Gametype3: gt3,
		})
	}
	return out
}

// readFog reads the global fog parameters (at RefAddrFogParams).
// Static per loaded map.
//
// Source: OffFog* constants.
func (r *Reader) readFog() *scraper.StaticFog {
	inst := r.inst
	mem := inst.Mem

	base, err := inst.LowHVA(RefAddrFogParams)
	if err != nil {
		return nil
	}

	r0, _ := mem.ReadF32At(base + int64(OffFogColorR))
	g0, _ := mem.ReadF32At(base + int64(OffFogColorG))
	b0, _ := mem.ReadF32At(base + int64(OffFogColorB))
	maxD, _ := mem.ReadF32At(base + int64(OffFogMaxDensity))
	minDist, _ := mem.ReadF32At(base + int64(OffFogAtmoMinDist))
	maxDist, _ := mem.ReadF32At(base + int64(OffFogAtmoMaxDist))

	return &scraper.StaticFog{
		ColorR:      r0,
		ColorG:      g0,
		ColorB:      b0,
		MaxDensity:  maxD,
		AtmoMinDist: minDist,
		AtmoMaxDist: maxDist,
	}
}

// objectTypeDefMaxScan caps how many entries we'll walk in the
// object-type-definition array. The engine has ~30-40 types in practice;
// cap conservatively to avoid runaway reads if the range bounds are bogus.
const objectTypeDefMaxScan = 64

// readObjectTypes walks the engine's object-type-def array between
// RefAddrObjectTypeDefRangeLo/Hi and returns one StaticObjectType per
// non-null entry. Static for the engine session.
//
// Source: OffObjTypeDef* + RefAddrObjectTypeDef* constants.
func (r *Reader) readObjectTypes() []scraper.StaticObjectType {
	inst := r.inst
	mem := inst.Mem

	loHVA, err := inst.LowHVA(RefAddrObjectTypeDefRangeLo)
	if err != nil {
		return nil
	}
	hiHVA, err := inst.LowHVA(RefAddrObjectTypeDefRangeHi)
	if err != nil {
		return nil
	}
	arrayHVA, err := inst.LowHVA(RefAddrObjectTypeDefArray)
	if err != nil {
		return nil
	}

	loVal, _ := mem.ReadU32At(loHVA)
	hiVal, _ := mem.ReadU32At(hiHVA)
	if hiVal <= loVal {
		return nil
	}
	count := int((hiVal - loVal) / 4)
	if count <= 0 {
		return nil
	}
	if count > objectTypeDefMaxScan {
		count = objectTypeDefMaxScan
	}

	out := make([]scraper.StaticObjectType, 0, count)
	for i := 0; i < count; i++ {
		entryHVA := arrayHVA + int64(i)*4
		typeDefPtr, _ := mem.ReadU32At(entryHVA)
		if typeDefPtr < HighGVAThreshold {
			continue
		}
		stringPtr, _ := mem.ReadU32(typeDefPtr + OffObjTypeDefStringPtr)
		datumSize, _ := mem.ReadU16(typeDefPtr + OffObjTypeDefDatumSize)

		name := ""
		if stringPtr >= HighGVAThreshold {
			name = r.readHighString(stringPtr)
		}

		out = append(out, scraper.StaticObjectType{
			TypeIndex: i,
			Name:      name,
			DatumSize: datumSize,
		})
	}
	return out
}

// readCachePtrs reads the four cache base/size pointer pairs (game state, tag,
// texture, sound). Diagnostic-only — surfaces in SnapshotPayload.TagCache for
// memory-layout introspection in the debug page.
func (r *Reader) readCachePtrs() *scraper.StaticCachePtrs {
	inst := r.inst
	mem := inst.Mem

	read := func(addr uint32) uint32 {
		hva, err := inst.LowHVA(addr)
		if err != nil {
			return 0
		}
		v, _ := mem.ReadU32At(hva)
		return v
	}

	return &scraper.StaticCachePtrs{
		GameStateBase:    read(RefAddrGameStateBasePtr),
		GameStateSize:    read(RefAddrGameStateSize),
		TagCacheBase:     read(RefAddrTagCacheBasePtr),
		TagCacheSize:     read(RefAddrTagCacheSize),
		TextureCacheBase: read(RefAddrTextureCacheBasePtr),
		TextureCacheSize: read(RefAddrTextureCacheSize),
		SoundCacheBase:   read(RefAddrSoundCacheBasePtr),
		SoundCacheSize:   read(RefAddrSoundCacheSize),
	}
}

// readGameDifficulty reads OffGGGameDifficultyLevel from game_globals.
// Returns 0 when game_globals isn't yet allocated.
func (r *Reader) readGameDifficulty() uint8 {
	inst := r.inst
	ggPtr, err := inst.DerefLowPtr(AddrGameGlobalsPtr)
	if err != nil || ggPtr < HighGVAThreshold {
		return 0
	}
	d, _ := inst.Mem.ReadU8(ggPtr + OffGGGameDifficultyLevel)
	return d
}
