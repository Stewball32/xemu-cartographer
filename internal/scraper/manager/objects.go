package manager

import "github.com/Stewball32/xemu-cartographer/internal/scraper"

// envelopeTypeObjects is the wire type for the per-instance objects class
// — the world-object firehose (vehicles, scenery, dropped weapons,
// grenades, etc.) and live projectiles. Per-tick (~30 Hz), opt-in
// subscription because an overlay HUD doesn't need 45+ objects every
// frame; only map views and replays subscribe.
//
// See atlas/new_json/04-ground-up-rebuild.md §6 (`objects`). Named after
// Halo's "object table" concept; projectiles are also `object_data`
// entries in the engine, so the class subsumes both.
const envelopeTypeObjects = "objects"

// ObjectsPayload is the data for an objects-class envelope.
type ObjectsPayload struct {
	Objects     []WorldObject `json:"objects"`
	Projectiles []Projectile  `json:"projectiles"`
}

type WorldObject struct {
	ObjectID       uint32       `json:"object_id"`
	Tag            string       `json:"tag"`
	Type           uint8        `json:"type"`
	Flags          uint32       `json:"flags"`
	Pos            scraper.Vec3 `json:"pos"`
	AngVel         scraper.Vec3 `json:"ang_vel"`
	TimeExisting   int16        `json:"time_existing"`
	OwnerUnit      *uint32      `json:"owner_unit"`
	OwnerObject    *uint32      `json:"owner_object"`
	UltimateParent uint32       `json:"ultimate_parent"`
}

type Projectile struct {
	ObjectID         uint32       `json:"object_id"`
	Tag              string       `json:"tag"`
	Pos              scraper.Vec3 `json:"pos"`
	Flags            uint32       `json:"flags"`
	Action           int16        `json:"action"`
	DetonationTimer  float32      `json:"detonation_timer"`
	DistanceTraveled float32      `json:"distance_traveled"`
}
