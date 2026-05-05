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
	}
	return nil
}

// ensureScenarioStatic fills the scenario-static cache once the scenario
// pointer is reachable. Idempotent. Leaves Filled=false on early-pregame
// pointer-not-yet-set so the next call retries.
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

	r.scenarioCache.MapName = r.readLowString(AddrMultiplayerMapName, 32)
	r.scenarioCache.GameDifficulty = r.readGameDifficulty()
	r.scenarioCache.PlayerSpawns = r.readPlayerSpawns()
	r.scenarioCache.Fog = r.readFog()
	r.scenarioCache.ObjectTypes = r.readObjectTypes()
	r.scenarioCache.TagCache = r.readCachePtrs()
	r.scenarioCache.PowerSpawnsScenario = r.readPowerSpawnScenarios()
	r.scenarioCache.Filled = true
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
	// running, so this typically fills on the first in_game tick.
	if !r.matchCache.InitialObjIDsFilled && r.scenarioCache != nil && r.scenarioCache.Filled {
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
