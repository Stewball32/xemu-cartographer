package manager

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// envelopeTypeScenario is the wire type for the per-instance scenario
// class — the loaded map. One message per map load; everything static
// lives here, including the per-tag definition table that tick references
// by string.
//
// See atlas/new_json/04-ground-up-rebuild.md §6 (`scenario`).
const envelopeTypeScenario = "scenario"

// ScenarioPayload is the data for a scenario-class envelope.
type ScenarioPayload struct {
	Map             string                    `json:"map"`
	GameDifficulty  uint8                     `json:"game_difficulty"`
	Fog             *ScenarioFog              `json:"fog"`
	MemoryRegions   *ScenarioMemoryRegions    `json:"memory_regions"`
	ObjectTypes     []ScenarioObjectType      `json:"object_types"`
	PlayerSpawns    []ScenarioPlayerSpawn     `json:"player_spawns"`
	PowerItemSpawns []ScenarioPowerItemSpawn  `json:"power_item_spawns"`
	TagDefs         map[string]ScenarioTagDef `json:"tag_defs"`
}

type ScenarioFog struct {
	Color       ScenarioFogColor `json:"color"`
	MaxDensity  float32          `json:"max_density"`
	AtmoMinDist float32          `json:"atmo_min_dist"`
	AtmoMaxDist float32          `json:"atmo_max_dist"`
}

type ScenarioFogColor struct {
	R float32 `json:"r"`
	G float32 `json:"g"`
	B float32 `json:"b"`
}

// ScenarioMemoryRegions replaces the misleadingly-named v1 `tag_cache`
// field — it's a set of memory base/size pointers, not a cache of tags.
type ScenarioMemoryRegions struct {
	GameState    ScenarioMemoryRegion `json:"game_state"`
	TagCache     ScenarioMemoryRegion `json:"tag_cache"`
	TextureCache ScenarioMemoryRegion `json:"texture_cache"`
	SoundCache   ScenarioMemoryRegion `json:"sound_cache"`
}

type ScenarioMemoryRegion struct {
	Base uint32 `json:"base"`
	Size uint32 `json:"size"`
}

type ScenarioObjectType struct {
	TypeIndex int    `json:"type_index"`
	Name      string `json:"name"`
	DatumSize uint16 `json:"datum_size"`
}

type ScenarioPlayerSpawn struct {
	Index     int          `json:"index"`
	Pos       scraper.Vec3 `json:"pos"`
	Facing    float32      `json:"facing"`
	TeamIndex uint8        `json:"team_index"`
	BspIndex  uint8        `json:"bsp_index"`
	Gametypes [4]uint8     `json:"gametypes"`
}

type ScenarioPowerItemSpawn struct {
	SpawnID       int          `json:"spawn_id"`
	Tag           string       `json:"tag"`
	IntervalTicks int16        `json:"interval_ticks"`
	Pos           scraper.Vec3 `json:"pos"`
}

// ScenarioTagDef is one entry in the tag_defs table. Discriminated union
// keyed on Kind — weapon and biped (today's only kinds) have disjoint
// fields; future kinds add fields with omitempty so they don't pollute the
// wire for tags that don't have them.
type ScenarioTagDef struct {
	Kind string `json:"kind"` // "weapon" | "biped"

	// weapon fields
	ZoomLevels     int16   `json:"zoom_levels,omitempty"`
	ZoomMin        float32 `json:"zoom_min,omitempty"`
	ZoomMax        float32 `json:"zoom_max,omitempty"`
	AutoaimAngle   float32 `json:"autoaim_angle,omitempty"`
	AutoaimRange   float32 `json:"autoaim_range,omitempty"`
	MagnetismAngle float32 `json:"magnetism_angle,omitempty"`
	MagnetismRange float32 `json:"magnetism_range,omitempty"`
	DeviationAngle float32 `json:"deviation_angle,omitempty"`

	// biped fields
	TagIndex          int16   `json:"tag_index,omitempty"`
	Flags             uint32  `json:"flags,omitempty"`
	AutoaimPillRadius float32 `json:"autoaim_pill_radius,omitempty"`
}
