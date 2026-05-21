package manager

import (
	"github.com/Stewball32/xemu-cartographer/internal/guards"
	"github.com/Stewball32/xemu-cartographer/internal/scraper"
)

// scenarioFingerprint is a packed snapshot of the observable scenario-
// derived field counts on a scraper.GameData. Used to detect when the
// underlying scenarioStaticCache (in the haloce reader) has filled or
// changed without exposing the reader's internals to the manager — the
// scenario fields in GameData are populated *only* from the cache
// (composeGameData in reader.go), so length transitions are a clean
// signal. Map is excluded because the low-GVA fallback there can
// populate it independently of the cache state.
//
// Layout (high → low): PlayerSpawns count (16 bits) | ObjectTypes count
// (16 bits) | PowerItemSpawns count (16 bits) | Fog present (1 bit) |
// TagCache present (1 bit) | reserved (14 bits). ObjectTypes can hit
// ~600 entries on Halo: CE; 16 bits leaves room for any plausible
// scenario.
type scenarioFingerprint uint64

// computeScenarioFingerprint packs the observable scenario field counts
// from gd into a fingerprint. Returns 0 for nil GameData or a GameData
// with no scenario-derived fields populated.
func computeScenarioFingerprint(gd *scraper.GameData) scenarioFingerprint {
	if gd == nil {
		return 0
	}
	var f uint64
	f |= uint64(uint16(len(gd.PlayerSpawns))) << 48
	f |= uint64(uint16(len(gd.ObjectTypes))) << 32
	f |= uint64(uint16(len(gd.PowerItemSpawns))) << 16
	if gd.Fog != nil {
		f |= 1
	}
	if gd.TagCache != nil {
		f |= 2
	}
	return scenarioFingerprint(f)
}

// maybeEmitScenario re-broadcasts the scenario class envelope when the
// observable scenario-derived fields in cache.GameData change. Edge-
// trigger for the "scenarioStaticCache fills lazily after Ready→Live
// broadcastSnapshot already shipped an empty scenario envelope" race
// (M6 audit). Called from the loop goroutine after every publishGameData
// site; broadcastSnapshot keeps the fingerprint in sync with its own
// emissions so phase transitions don't double-emit.
//
// TagDefs on the scenario payload come from cache.LatestTick (per-player
// weapon tag_data), not from GameData — the fingerprint doesn't track
// them. Mid-match weapon-pickup tag-def additions wait for the next
// scenario-field change to ride through. Acceptable: TagDefs are a
// debug-page nicety, not gameplay-critical.
func (r *runner) maybeEmitScenario(svc *guards.Services) {
	if svc == nil || svc.WS == nil {
		return
	}
	c := r.readCache()
	fp := computeScenarioFingerprint(c.GameData)
	if fp == r.lastScenarioFingerprint {
		return
	}
	r.lastScenarioFingerprint = fp
	sp := buildScenarioPayload(&c)
	if sp == nil {
		return
	}
	r.emitClass(svc, "scenario", c.EngineTick, *sp)
}
