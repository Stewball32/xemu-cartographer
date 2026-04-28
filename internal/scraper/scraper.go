// Package scraper defines the interface for game-specific memory scrapers and
// provides auto-detection of which game is running inside an xemu instance.
//
// Game implementations (e.g. internal/scraper/haloce) register themselves via
// init() + Register(). The poll loop in cmd/cartographer uses Detect() to pick
// the right implementation at connect time.
package scraper

import (
	"fmt"
	"sync"

	"github.com/Stewball32/xemu-cartographer/internal/xemu"
)

// GameReader is the contract every game plugin must implement.
type GameReader interface {
	// LowGVAs returns the guest virtual addresses that need QMP translation
	// at Init time for this game.
	LowGVAs() []uint32

	// ReadGameState is the lightweight poll-loop check.
	ReadGameState() (GameState, uint32, error)

	// ReadSnapshot reads full game state for state transitions and new connections.
	ReadSnapshot() (SnapshotPayload, error)

	// ReadTick reads per-tick dynamic state.
	ReadTick(spawns []PowerItemSpawn, state *TickState) (TickResult, error)

	// DetectEvents compares current tick against previous state, returns events.
	DetectEvents(tick uint32, instance string, snap SnapshotPayload, result TickResult, state *TickState) []Envelope

	// NewTickState returns a fresh tick state tracker.
	NewTickState() *TickState

	// XboxName returns the console name of the xbox running this game, or ""
	// when the plugin can't resolve it (e.g. Halo 2 has no known offset).
	XboxName() string

	// Title is the human-readable game title (e.g. "Halo: Combat Evolved").
	Title() string
}

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

var (
	registryMu sync.Mutex
	registry   = map[uint32]Factory{}
)

// Factory creates a GameReader for a given xemu instance.
type Factory func(inst *xemu.Instance, instanceName string) GameReader

// Register associates an Xbox title ID with a GameReader factory. Game packages
// call this from their init() function.
func Register(titleID uint32, f Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[titleID] = f
}

// Lookup returns the factory for a title ID, or nil if unknown.
func Lookup(titleID uint32) Factory {
	registryMu.Lock()
	defer registryMu.Unlock()
	return registry[titleID]
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
// matching GameReader. Returns an error if the title ID is unrecognised.
func Detect(inst *xemu.Instance, instanceName string) (GameReader, uint32, error) {
	// Read the certificate pointer from the XBE header.
	headerHVA, err := inst.LowHVA(xbeHeaderGVA)
	if err != nil {
		return nil, 0, fmt.Errorf("detect: translate XBE header: %w", err)
	}
	certPtr, err := inst.Mem.ReadU32At(headerHVA + int64(xbeOffCertPtr))
	if err != nil {
		return nil, 0, fmt.Errorf("detect: read certificate pointer: %w", err)
	}

	// Compute host VA for the certificate. Low GVAs are relative to
	// the already-translated header page; high GVAs use the standard offset.
	var certHVA int64
	if certPtr < 0x80000000 {
		certHVA = headerHVA + int64(certPtr) - int64(xbeHeaderGVA)
	} else {
		certHVA = inst.Mem.HighGVA(certPtr)
	}

	titleID, err := inst.Mem.ReadU32At(certHVA + int64(xbeCertOffTitleID))
	if err != nil {
		return nil, 0, fmt.Errorf("detect: read title ID: %w", err)
	}

	factory := Lookup(titleID)
	if factory == nil {
		return nil, titleID, fmt.Errorf("detect: unknown title ID 0x%08X", titleID)
	}

	return factory(inst, instanceName), titleID, nil
}
