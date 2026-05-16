package scraper

// Sub-shapes referenced across v2 state-class payloads and event payloads.
// Defined in the scraper package (rather than manager) so the haloce events
// subpackage can use them without an import cycle.
//
// See atlas/new_json/04-ground-up-rebuild.md §6 (sub-shapes) and the pos-
// field-discipline table for which events carry positions and how.

// Vec3 is the canonical 3D vector type used across v2 — positions,
// velocities, aim directions, etc. Alias for the legacy XYZ so existing v1
// code (PowerItemStatus.WorldPos, TickWeaponObjectExtended.World) keeps
// compiling during the rebuild window.
type Vec3 = XYZ

// Vec2 is the canonical 2D vector — joystick coordinates, screen-space
// positions, etc.
type Vec2 struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
}

// PlayerRef carries identity-only references to a player at the moment of
// an event. Positions are NOT part of the ref — they live as peer fields on
// each event (`pos`, `victim_pos`, `killer_pos`, `dealer_pos`) so the ref
// shape stays identical across every event type.
//
// Identity (name, team, armor_color) is denormalized onto the ref so an
// event log alone is sufficient for analytics — a team switch later in the
// game does not retroactively change what was true at the moment of the
// event.
type PlayerRef struct {
	Index      int    `json:"index"`
	Name       string `json:"name"`
	Team       uint32 `json:"team"`
	ArmorColor int16  `json:"armor_color"`
}

// ItemRef carries identity-only references to a world item (power-item
// spawn, weapon, etc.). Position is a peer field on the carrying event when
// relevant.
//
// SpawnID is `omitempty` because some items aren't tied to a scenario spawn
// (e.g. weapons dropped on death have no spawn_id).
type ItemRef struct {
	SpawnID int    `json:"spawn_id,omitempty"`
	Tag     string `json:"tag"`
}

// VehicleRef carries identity-only references to a vehicle object. Seat
// (driver / passenger / gunner) is a peer field on the vehicle_entered /
// vehicle_exited event payloads rather than part of the ref, since "seat"
// is event-specific context, not vehicle identity.
type VehicleRef struct {
	ObjectID uint32 `json:"object_id"`
	Tag      string `json:"tag"`
}
