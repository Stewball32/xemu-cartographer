package haloce

import (
	"log"

	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// readPlayerSpawns walks the scenario's player-spawn array and returns one
// StaticPlayerSpawn per entry. Reads at game-data read time only (scenario data is
// map-static).
//
// Source: OffScenarioPlayerSpawn* + OffPlayerSpawn* constants.
func (r *Reader) readPlayerSpawns() []scraper.StaticPlayerSpawn {
	inst := r.inst
	mem := inst.Mem

	scenarioBase, err := inst.DerefLowPtr(r.off.AddrGlobalScenarioPtr)
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

	base, err := inst.LowHVA(r.off.RefAddrFogParams)
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
// object-type-definition array. The engine has 11 types on Xbox; cap a bit
// higher in case a different build adds entries, and to bound runaway
// reads if the range bounds ever land on garbage.
const objectTypeDefMaxScan = 64

// haloceObjectTypeNames is the canonical name per type_index, in the
// order Halo CE's engine stores them. Hardcoded because the per-type
// struct's +0x00 field is a self-pointer/vtable on Xbox (no string
// pointer, no fourcc), and the type-table order is fixed by the engine.
var haloceObjectTypeNames = [11]string{
	"biped",
	"vehicle",
	"weapon",
	"equipment",
	"garbage",
	"projectile",
	"scenery",
	"machine",
	"control",
	"light_fixture",
	"placeholder",
}

// readObjectTypes walks the engine's object-type-def array (at
// RefAddrObjectTypeDefArray, length = (RangeHi - Array) / 4 = 11 on Xbox)
// and returns one StaticObjectType per non-null entry. Static for the
// engine session.
//
// Halocaster.py:344's RefAddrObjectTypeDefRangeLo was previously read as
// a u32 pointer alongside Hi to compute the array count, but on Xbox both
// addresses are literal array bounds, not pointers — Lo (= 0x1FC0D0) lands
// inside a string region containing "object\0…" and dereferencing it gave
// nonsense. Count is now computed from the constants directly; Lo deleted.
//
// Per-entry field at OffObjTypeDefClassFourcc is the engine's 4-char class
// tag stored in reverse byte order (e.g. memory 'd','p','i','b' → "bipd"),
// not a string pointer as halocaster.py believed.
//
// One-shot diagnostic log fires on the first call per Reader (M19) to
// surface which gate trips when the result is empty.
func (r *Reader) readObjectTypes() []scraper.StaticObjectType {
	inst := r.inst
	mem := inst.Mem

	count := int((r.off.RefAddrObjectTypeDefRangeHi - r.off.RefAddrObjectTypeDefArray) / 4)
	if count > objectTypeDefMaxScan {
		count = objectTypeDefMaxScan
	}
	var filteredByPtr, kept int
	gate := "ok"
	defer func() {
		if r.loggedObjectTypesDiag {
			return
		}
		r.loggedObjectTypesDiag = true
		log.Printf("scraper[%s]: readObjectTypes diag: gate=%s count=%d filteredByPtr=%d kept=%d",
			r.name, gate, count, filteredByPtr, kept)
	}()

	arrayHVA, err := inst.LowHVA(r.off.RefAddrObjectTypeDefArray)
	if err != nil {
		gate = "LowHVA(Array)"
		return nil
	}

	out := make([]scraper.StaticObjectType, 0, count)
	for i := 0; i < count; i++ {
		entryHVA := arrayHVA + int64(i)*4
		typeDefPtr, _ := mem.ReadU32At(entryHVA)
		if typeDefPtr == 0 {
			filteredByPtr++
			continue
		}

		// Type-def structs live in the engine's .data section at LOW GVA
		// (typically 0x001Fxxxx, in the same low-memory page as the array).
		// mem.ReadU32(gva) assumes high GVA; for low GVAs we must translate
		// via QMP (cached after the first call by lowHVAs).
		var typeDefHVA int64
		if typeDefPtr >= HighGVAThreshold {
			typeDefHVA = mem.HighGVA(typeDefPtr)
		} else {
			hva, err := inst.LowHVA(typeDefPtr)
			if err != nil {
				hva, err = inst.RefreshLowHVA(typeDefPtr)
			}
			if err != nil {
				filteredByPtr++
				continue
			}
			typeDefHVA = hva
		}
		datumSize, _ := mem.ReadU16At(typeDefHVA + int64(OffObjTypeDefDatumSize))

		name := ""
		if i < len(haloceObjectTypeNames) {
			name = haloceObjectTypeNames[i]
		}

		out = append(out, scraper.StaticObjectType{
			TypeIndex: i,
			Name:      name,
			DatumSize: datumSize,
		})
		kept++
	}
	if kept == 0 {
		gate = "allFilteredByPtr"
	}
	return out
}

// readCachePtrs reads the four cache base/size pointer pairs (game state, tag,
// texture, sound). Diagnostic-only — surfaces in GameData.TagCache for
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
		GameStateBase:    read(r.off.RefAddrGameStateBasePtr),
		GameStateSize:    read(r.off.RefAddrGameStateSize),
		TagCacheBase:     read(r.off.RefAddrTagCacheBasePtr),
		TagCacheSize:     read(r.off.RefAddrTagCacheSize),
		TextureCacheBase: read(r.off.RefAddrTextureCacheBasePtr),
		TextureCacheSize: read(r.off.RefAddrTextureCacheSize),
		SoundCacheBase:   read(r.off.RefAddrSoundCacheBasePtr),
		SoundCacheSize:   read(r.off.RefAddrSoundCacheSize),
	}
}

// readGameDifficulty reads OffGGGameDifficultyLevel from game_globals.
// Returns 0 when game_globals isn't yet allocated.
func (r *Reader) readGameDifficulty() uint8 {
	inst := r.inst
	ggPtr, err := inst.DerefLowPtr(r.off.AddrGameGlobalsPtr)
	if err != nil || ggPtr < HighGVAThreshold {
		return 0
	}
	d, _ := inst.Mem.ReadU8(ggPtr + OffGGGameDifficultyLevel)
	return d
}
