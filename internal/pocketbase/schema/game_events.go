package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerGameEventsCollection)
}

// registerGameEventsCollection creates the game_events collection — the
// canonical PB destination for `pb:game_events` capture-policy sinks
// (see atlas/new_json/04-ground-up-rebuild.md §5).
//
// Each row is one v2 event envelope. The flat columns (instance, type,
// seq, tick, ts) cover the indexable / orderable dimensions; the inner
// payload sits in `data` as JSON so queries can drill into
// event_type / killer_index / pos etc. without re-shaping the schema
// every time a new event field appears.
//
// Indexes prioritise the two read patterns: "events for instance N
// since seq S" and "events of semantic type T in instance N".
func registerGameEventsCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "game_events") {
		return nil
	}

	collection := core.NewBaseCollection("game_events")

	collection.Fields.Add(
		&core.TextField{
			Name:        "instance",
			Required:    true,
			Min:         1,
			Max:         64,
			Presentable: true,
		},
		&core.TextField{
			Name:        "type",
			Required:    true,
			Min:         1,
			Max:         32,
			Presentable: true,
		},
		&core.NumberField{
			Name:    "seq",
			OnlyInt: true,
			Min:     f64(0),
		},
		&core.NumberField{
			Name:    "tick",
			OnlyInt: true,
			Min:     f64(0),
		},
		&core.DateField{
			Name: "ts",
		},
		&core.JSONField{
			Name:    "data",
			MaxSize: 1 << 20, // 1 MiB — events are small, give headroom for nested arrays
		},
		&core.AutodateField{
			Name:     "created",
			OnCreate: true,
		},
	)

	// Replay scrubbing: walk events for one runner from a known seq forward.
	collection.AddIndex("idx_game_events_instance_seq", false, "instance, seq", "")
	// Semantic filters: kills only, medals only, etc.
	collection.AddIndex("idx_game_events_instance_type", false, "instance, type", "")

	// Admin-only via the dashboard; future read endpoints can surface
	// filtered views. No public REST CRUD.
	collection.ListRule = nil
	collection.ViewRule = nil
	collection.CreateRule = nil
	collection.UpdateRule = nil
	collection.DeleteRule = nil

	return app.Save(collection)
}
