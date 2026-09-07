package manager

import "github.com/xemu-cartographer/xc-scraper/wire"

// The scenario class payload shapes are owned by the wire contract package
// (xc-scraper/wire/scenario.go); the names below are aliases so the builders
// in v2_adapters.go and every consumer keep one type identity.
//
// See atlas/new_json/04-ground-up-rebuild.md §6 (`scenario`).
type (
	// ScenarioPayload is the data for a scenario-class envelope — the loaded
	// map. One message per map load; everything static lives here,
	// including the per-tag definition table that tick references by string.
	ScenarioPayload        = wire.ScenarioPayload
	ScenarioFog            = wire.ScenarioFog
	ScenarioFogColor       = wire.ScenarioFogColor
	ScenarioMemoryRegions  = wire.ScenarioMemoryRegions
	ScenarioMemoryRegion   = wire.ScenarioMemoryRegion
	ScenarioObjectType     = wire.ScenarioObjectType
	ScenarioPlayerSpawn    = wire.ScenarioPlayerSpawn
	ScenarioPowerItemSpawn = wire.ScenarioPowerItemSpawn
	// ScenarioTagDef is one entry in the tag_defs table — a discriminated
	// union keyed on Kind ("weapon" | "biped").
	ScenarioTagDef = wire.ScenarioTagDef
)
