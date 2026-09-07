// Package scraper defines the interface for game-specific memory scrapers and
// provides auto-detection of which game is running inside an xemu instance.
//
// Game implementations (e.g. internal/scraper/haloce) register themselves via
// init() + Register(). The poll loop in cmd/cartographer uses Detect() to pick
// the right implementation at connect time.
//
// Adding a second game profile (e.g. Halo 2)
// ------------------------------------------
// The seam is title-ID-keyed and fully decoupled — no existing file changes:
//
//  1. Create internal/scraper/halo2/ with its own offsets.go (the game's
//     verified offset constants — game-specific values live ONLY in that
//     package, never in shared code) and a Reader implementing GameReader.
//  2. Register the H2 title ID in the package's init() via scraper.Register,
//     and blank-import the package from cmd/server/main.go so init() runs.
//  3. Reuse the game-agnostic wire structs in this package (GameData,
//     TickPlayer, TickProjectile, …). Their fields — health/shields/max,
//     frags, scores, arming_time, etc. — are not CE-specific; H2 populates the
//     same shapes from its own offsets. Add a field only if H2 surfaces
//     something genuinely new.
//
// Offsets flow from the halo-offset-mapper export (export_cartographer.py) →
// reconciled by hand into the game package's offsets.go (see that repo's
// docs/CARTOGRAPHER-IMPORT.md). H2's export is pending; wire it when it lands.
package scraper

import (
	"fmt"
	"log"
	"sync"

	"github.com/Stewball32/xemu-cartographer/internal/scraper/offsets"
	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// GameReader is the contract every game plugin must implement.
type GameReader interface {
	// LowGVAs returns the guest virtual addresses that need QMP translation
	// at Init time for this game.
	LowGVAs() []uint32

	// ReadGameState is the lightweight poll-loop check.
	ReadGameState() (GameState, uint32, error)

	// LastStateInputs returns the raw values sampled by the most recent
	// ReadGameState call. Used by the inspect endpoint for diagnostics. Plugins
	// that don't track state inputs may return nil.
	LastStateInputs() StateInputs

	// BuildScoreProbe reads every candidate address the plugin knows about
	// for gametype/team-score/per-player-score detection and returns a free-
	// form bag of the raw values. Called by the manager loop and surfaced to
	// the debug page's Probe tab. May read memory; called from the scraper
	// goroutine only. Plugins that don't have score logic may return nil.
	BuildScoreProbe() ScoreProbe

	// ReadGameData reads the full game-data field set: scenario-static
	// (map, spawns, fog), match-static (gametype, score limit, rosters),
	// and live-volatile (current scores, player counters). Called once on
	// the Ready→Live transition (match-static fields are then cached in
	// the runner's instanceCache for the rest of the match) and as the
	// "current state" payload returned by ReadReadyState.
	//
	// Implementations should serve as much from cached scenario / match
	// static state as possible and only re-read live-volatile fields on
	// each call.
	ReadGameData() (GameData, error)

	// ReadReadyState is the cheap variant of ReadGameData intended to
	// be called every loop iteration in the Ready phase (lobby / pregame
	// / postgame / between-match menu). Same return type and semantics —
	// the name distinguishes the call site so the loop reads as "refresh
	// the ready-phase view."
	ReadReadyState() (GameData, error)

	// ReadTick reads per-tick dynamic state.
	ReadTick(spawns []PowerItemSpawn, state *TickState) (TickResult, error)

	// DetectEvents compares current tick against previous state, returns events.
	DetectEvents(tick uint32, instance string, snap GameData, result TickResult, state *TickState) []Envelope

	// OnStateChange is invoked by the loop on every detected state transition.
	// Implementations use it to invalidate scenario- or match-scoped caches.
	// Called with prev=="" on the first observed state.
	OnStateChange(prev, next GameState) error

	// NewTickState returns a fresh tick state tracker.
	NewTickState() *TickState

	// Title is the human-readable game title (e.g. "Halo: Combat Evolved").
	//
	// Console name is NOT a plugin concern — that's an Xbox-the-machine fact
	// supplied by internal/scraper/xbox/ ReadConsoleName, populated by the
	// runner's system-snapshot pass independently of which game is bound.
	Title() string
}

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

var (
	registryMu sync.Mutex
	registry   = map[uint32]registration{}
)

// Factory creates a GameReader for a given xemu instance, binding the given
// versioned offset set (the game's baseline unless the catalog assigns another
// set). A factory errors when the set can't be bound (missing keys).
type Factory func(inst *xemu.Instance, instanceName string, set *offsets.Set) (GameReader, error)

// registration pairs a factory with its game key (the offsets-layer game id,
// e.g. "haloce" / "halo2") so Detect can resolve the right offset set.
type registration struct {
	gameKey string
	factory Factory
}

// Register associates an Xbox title ID with a game key + GameReader factory.
// Game packages call this from their init() function. gameKey is the offsets
// package game id the title's offset sets are registered under.
func Register(titleID uint32, gameKey string, f Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[titleID] = registration{gameKey: gameKey, factory: f}
}

// Lookup returns the factory for a title ID, or nil if unknown.
func Lookup(titleID uint32) Factory {
	registryMu.Lock()
	defer registryMu.Unlock()
	return registry[titleID].factory
}

// ---------------------------------------------------------------------------
// Detection
// ---------------------------------------------------------------------------

// XBE header constants for title ID detection.
const (
	// The XBE header is loaded at GVA 0x00010000 on the original Xbox.
	xbeHeaderGVA uint32 = 0x00010000

	// Offset within the XBE header to the certificate pointer (u32).
	xbeOffCertPtr uint32 = 0x0118

	// Offset within the XBE certificate to the title ID (u32).
	xbeCertOffTitleID uint32 = 0x0008
)

// DetectionGVAs returns the low guest VAs needed for game detection. Pass these
// to xemu.Instance.Init() before calling Detect().
func DetectionGVAs() []uint32 {
	return []uint32{xbeHeaderGVA}
}

// Detect reads the XBE title ID from the running Xbox game and returns the
// matching GameReader, bound to the offset set named by offsetSetID (empty =
// the game's baseline). An unknown/mismatched explicit id degrades to the
// baseline with a logged warning — a bad data assignment must not stop a
// vanilla box; a set that exists but fails to BIND is a hard error (that set
// is genuinely unusable).
func Detect(inst *xemu.Instance, instanceName, offsetSetID string) (GameReader, uint32, error) {
	titleID, err := ReadTitleID(inst)
	if err != nil {
		return nil, 0, err
	}
	reader, err := NewReaderForTitle(titleID, inst, instanceName, offsetSetID)
	if err != nil {
		return nil, titleID, err
	}
	return reader, titleID, nil
}

// NewReaderForTitle binds a GameReader for an already-detected title ID,
// resolving the offset set named by offsetSetID (empty = the game's baseline).
// An unknown/mismatched explicit id degrades to the baseline with a logged
// warning; a set that exists but fails to BIND is a hard error, as is a game
// with no registered baseline (never a panic — the runner stays alive).
func NewReaderForTitle(titleID uint32, inst *xemu.Instance, instanceName, offsetSetID string) (GameReader, error) {
	registryMu.Lock()
	reg, ok := registry[titleID]
	registryMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("detect: unknown title ID 0x%08X", titleID)
	}
	set, warn := offsets.Resolve(reg.gameKey, offsetSetID)
	if set == nil {
		return nil, fmt.Errorf("resolve offset set for %s: %w", reg.gameKey, warn)
	}
	if warn != nil {
		log.Printf("scraper[%s]: offset set: %v", instanceName, warn)
	} else if offsetSetID != "" {
		log.Printf("scraper[%s]: using offset set %q for %s", instanceName, set.ID, reg.gameKey)
	}
	reader, err := reg.factory(inst, instanceName, set)
	if err != nil {
		return nil, fmt.Errorf("bind offset set %q: %w", set.ID, err)
	}
	return reader, nil
}

// ReadTitleID reads the running XBE's title ID via the same XBE header /
// certificate path as Detect, but without registry lookup. Used by the
// manager's Idle / Ready phase title-ID polling — when the user inserts a
// game (dashboard → game) or quits (game → dashboard / game → game), the
// guest VA 0x00010000 stays valid but the underlying physical page moves.
// Always re-translates the XBE header GVA via QMP before reading so the
// caller sees the *current* XBE's title ID rather than stale bytes from
// the previous mapping.
func ReadTitleID(inst *xemu.Instance) (uint32, error) {
	headerHVA, err := inst.RefreshLowHVA(xbeHeaderGVA)
	if err != nil {
		return 0, fmt.Errorf("detect: translate XBE header: %w", err)
	}
	certPtr, err := inst.Mem.ReadU32At(headerHVA + int64(xbeOffCertPtr))
	if err != nil {
		return 0, fmt.Errorf("detect: read certificate pointer: %w", err)
	}
	var certHVA int64
	if certPtr < 0x80000000 {
		certHVA = headerHVA + int64(certPtr) - int64(xbeHeaderGVA)
	} else {
		certHVA = inst.Mem.HighGVA(certPtr)
	}
	titleID, err := inst.Mem.ReadU32At(certHVA + int64(xbeCertOffTitleID))
	if err != nil {
		return 0, fmt.Errorf("detect: read title ID: %w", err)
	}
	return titleID, nil
}
