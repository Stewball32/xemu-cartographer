package manager

import "github.com/xemu-cartographer/xc-scraper/wire"

// The objects class payload shapes are owned by the wire contract package
// (xc-scraper/wire/objects.go); the names below are aliases so the builders
// in v2_adapters.go and every consumer keep one type identity.
//
// See atlas/new_json/04-ground-up-rebuild.md §6 (`objects`).
type (
	// ObjectsPayload is the data for an objects-class envelope — the
	// world-object firehose (vehicles, scenery, dropped weapons, grenades,
	// etc.) and live projectiles. Per-tick (~30 Hz), opt-in subscription.
	ObjectsPayload = wire.ObjectsPayload
	WorldObject    = wire.WorldObject
	Projectile     = wire.Projectile
)
