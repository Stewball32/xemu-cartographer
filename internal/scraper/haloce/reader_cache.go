package haloce

import (
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// scenarioStaticCache holds data that's static for the lifetime of a loaded
// scenario (map). Filled lazily on the first non-menu state read, dropped on
// menu re-entry (which is the only way a different scenario can be loaded).
type scenarioStaticCache struct {
	Filled              bool
	MapName             string
	GameDifficulty      uint8
	PlayerSpawns        []scraper.StaticPlayerSpawn
	Fog                 *scraper.StaticFog
	ObjectTypes         []scraper.StaticObjectType
	TagCache            *scraper.StaticCachePtrs
	PowerSpawnsScenario []scenarioPowerSpawn
}

// scenarioPowerSpawn is the scenario-static portion of a power item spawn —
// position + tag + respawn interval. The match-static InitialObjectID lives
// in matchStaticCache.PowerInitialOIDs and is merged at composeGameData time.
type scenarioPowerSpawn struct {
	SpawnID            int
	Tag                string
	SpawnIntervalTicks int16
	GametypeMask       uint8 // u8 bitmask at scenario_item+0x04; which gametypes this placement applies to
	X, Y, Z            float32
}

// matchStaticCache holds per-match data that's locked once the match begins.
// Per-local UI / look rates fill on first ensureMatchStatic; if a splitscreen
// player joins mid-pregame, LocalCount changes and the per-local slices are
// rebuilt. Power-item InitialObjectIDs are gated separately because they
// require the world-object header to be populated (only true once gameplay
// is running).
//
// Match-config fields (is_team_game, gametype, score_limit) intentionally do
// NOT live here — they're cheap to read and the host can change them in
// pregame, so composeGameData reads them live each call.
type matchStaticCache struct {
	Filled              bool
	InitialObjIDsFilled bool
	LocalCount          uint16
	UI                  []*scraper.TickUIGlobals
	LookYawRate         []float32
	LookPitchRate       []float32
	CTFFlagBases        []scraper.TickCTFFlag
	PowerInitialOIDs    map[int]uint32 // spawn_id → InitialObjectID
}

// OnStateChange is called by the manager loop on every detected game-state
// transition. Implements the cache invalidation policy: any transition into
// menu drops scenario- and match-scoped caches because that's the only path
// by which a different scenario can be loaded next.
func (r *Reader) OnStateChange(prev, next scraper.GameState) error {
	if next == scraper.GameStateMenu {
		r.scenarioCache = nil
		r.matchCache = nil
		// Pointer bases and tag caches are scenario-scoped too. ensureBases
		// re-fetches them on the next read; the maps are repopulated lazily.
		r.tagInstBase = 0
		r.ohdBase = 0
		r.tagNameCache = make(map[int16]string)
		r.weaponTagDataCache = make(map[int16]*scraper.StaticWeaponTagData)
		r.bipedTagCache = make(map[int16]*scraper.StaticBipedTagData)
		// The UI cache reloads on entry to the front-end, so the SELECT MAP /
		// SELECT GAMETYPE widget-def handles must be re-resolved (see widget.go).
		r.lobbyCursorHandles = nil
	}
	return nil
}

// ensureScenarioStatic fills the scenario-static cache once the scenario
// pointer is reachable. Idempotent. Each field is re-read while still empty,
// because readers like readObjectTypes / readPowerSpawnScenarios depend on
// engine state (object-type cache range, tagInstBase) that may not be warm
// at the same moment scenarioBase becomes reachable. Filled flips true only
// once all critical readers have produced data, so a transient empty result
// on the first call doesn't get locked in permanently.
func (r *Reader) ensureScenarioStatic() {
	if r.scenarioCache != nil && r.scenarioCache.Filled {
		return
	}
	if r.scenarioCache == nil {
		r.scenarioCache = &scenarioStaticCache{}
	}

	scenarioBase, err := r.inst.DerefLowPtr(AddrGlobalScenarioPtr)
	if err != nil || scenarioBase < HighGVAThreshold {
		return
	}

	if r.scenarioCache.MapName == "" {
		r.scenarioCache.MapName = r.readScenarioTagName()
	}
	r.scenarioCache.GameDifficulty = r.readGameDifficulty()
	if len(r.scenarioCache.PlayerSpawns) == 0 {
		r.scenarioCache.PlayerSpawns = r.readPlayerSpawns()
	}
	if r.scenarioCache.Fog == nil {
		r.scenarioCache.Fog = r.readFog()
	}
	if r.scenarioCache.TagCache == nil {
		r.scenarioCache.TagCache = r.readCachePtrs()
	}
	if len(r.scenarioCache.ObjectTypes) == 0 {
		r.scenarioCache.ObjectTypes = r.readObjectTypes()
	}
	if len(r.scenarioCache.PowerSpawnsScenario) == 0 {
		r.scenarioCache.PowerSpawnsScenario = r.readPowerSpawnScenarios()
	}

	if r.scenarioCache.MapName != "" &&
		len(r.scenarioCache.PlayerSpawns) > 0 &&
		len(r.scenarioCache.ObjectTypes) > 0 &&
		len(r.scenarioCache.PowerSpawnsScenario) > 0 {
		r.scenarioCache.Filled = true
	}
}

// ensureMatchStatic fills the per-match cache. Two-phase: most fields fill on
// the first call once any local has signed in; InitialObjectIDs require the
// world-object header to be populated and is gated separately so it can fill
// later (typically right after pregame → in_game).
//
// Mid-pregame splitscreen joins are handled by re-reading LocalCount each
// call and rebuilding the per-local slices when it changes.
func (r *Reader) ensureMatchStatic() {
	if r.matchCache == nil {
		r.matchCache = &matchStaticCache{}
	}

	if !r.matchCache.Filled {
		r.matchCache.LocalCount = r.readLocalPlayerCount()
		r.fillLocalsStatic(r.matchCache)
		r.matchCache.CTFFlagBases = r.readCTFFlags()
		r.matchCache.Filled = true
	} else {
		// Mid-pregame splitscreen-join detection. Cheap (1 deref + 1 u16 read).
		if cur := r.readLocalPlayerCount(); cur != r.matchCache.LocalCount {
			r.matchCache.LocalCount = cur
			r.fillLocalsStatic(r.matchCache)
		}
	}

	// InitialObjectIDs: gated on (a) scenario power-spawn list available and
	// (b) world-object header populated. (b) is only true once gameplay is
	// running, so this typically fills on the first in_game tick. Gated on
	// PowerSpawnsScenario directly rather than scenarioCache.Filled — the
	// Filled flag also depends on ObjectTypes populating, which is unrelated
	// to power-item OIDs and may lag or fail independently.
	if !r.matchCache.InitialObjIDsFilled && r.scenarioCache != nil && len(r.scenarioCache.PowerSpawnsScenario) > 0 {
		if r.ohdBase >= HighGVAThreshold {
			objHeaderFirst, _ := r.inst.Mem.ReadU32(r.ohdBase + OffOHDFirstElement)
			if objHeaderFirst >= HighGVAThreshold {
				r.matchCache.PowerInitialOIDs = r.readPowerInitialOIDs(r.scenarioCache.PowerSpawnsScenario)
				r.matchCache.InitialObjIDsFilled = true
			}
		}
	}
}

// fillLocalsStatic (re)builds the per-local slices to LocalCount entries.
// Called on first match-static fill and on mid-pregame splitscreen-join.
func (r *Reader) fillLocalsStatic(m *matchStaticCache) {
	n := int(m.LocalCount)
	if n > MaxLocalPlayers {
		n = MaxLocalPlayers
	}
	m.UI = make([]*scraper.TickUIGlobals, n)
	m.LookYawRate = make([]float32, n)
	m.LookPitchRate = make([]float32, n)
	for i := 0; i < n; i++ {
		m.UI[i] = r.readUIGlobals(i)
		m.LookYawRate[i] = r.readLookRate(RefAddrLookYawRate, i)
		m.LookPitchRate[i] = r.readLookRate(RefAddrLookPitchRate, i)
	}
}
