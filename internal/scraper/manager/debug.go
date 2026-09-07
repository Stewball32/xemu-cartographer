package manager

import "github.com/xemu-cartographer/xc-scraper/wire"

// The debug class payload shapes are owned by the wire contract package
// (xc-scraper/wire/debug.go); the names below are aliases so the builders
// in v2_adapters.go and every consumer keep one type identity.
//
// state_inputs / score_probe used to ride on this envelope; both moved
// to the on-demand probe class (see probe.go) so probe work doesn't
// run per-tick.
//
// See atlas/new_json/04-ground-up-rebuild.md §6 (`debug`) and §10 (the
// raw convention).
type (
	// DebugPayload is the data for a debug-class envelope — opt-in, only
	// read+emitted when a consumer (WS or capture policy) demands it.
	DebugPayload            = wire.DebugPayload
	DebugPlayer             = wire.DebugPlayer
	DebugPlayerExtended     = wire.DebugPlayerExtended
	DebugLegRotation        = wire.DebugLegRotation
	DebugSphere             = wire.DebugSphere
	DebugBone               = wire.DebugBone
	DebugUpdateQueue        = wire.DebugUpdateQueue
	DebugUpdateQueueButtons = wire.DebugUpdateQueueButtons
)
