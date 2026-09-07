package manager

import "github.com/xemu-cartographer/xc-scraper/wire"

// The tick class payload shapes are owned by the wire contract package
// (xc-scraper/wire/tick.go); the names below are aliases so the builders in
// v2_adapters.go and every consumer keep one type identity.
//
// See atlas/new_json/04-ground-up-rebuild.md §6 (`tick`).
type (
	// TickPayload is the data for a tick-class envelope — the hot path,
	// ~30 Hz, only volatile per-frame data. Static per-tag data is
	// referenced by string ID and lives in scenario.tag_defs.
	TickPayload           = wire.TickPayload
	TickPlayer            = wire.TickPlayer
	TickPlayerActions     = wire.TickPlayerActions
	TickWeapon            = wire.TickWeapon
	TickPowerItem         = wire.TickPowerItem
	TickCTFFlag           = wire.TickCTFFlag
	TickGameGlobals       = wire.TickGameGlobals
	TickLocal             = wire.TickLocal
	TickFPWeapon          = wire.TickFPWeapon
	TickObserverCam       = wire.TickObserverCam
	TickLocalInput        = wire.TickLocalInput
	TickLocalInputButtons = wire.TickLocalInputButtons
	TickPlayerControl     = wire.TickPlayerControl
)
