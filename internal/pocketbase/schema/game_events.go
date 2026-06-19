package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// Registration is coordinated from identity.go (M13 option-a): game_events now
// carries an optional `game` relation, so `games` must register first. The
// collection keeps its own create path here but no longer has a per-file
// init() — identity.go phase 4 calls it after games.

// registerGameEventsCollection creates (or reconciles) the game_events
// collection — the canonical PB destination for `pb:game_events` capture-policy
// sinks (see atlas/new_json/04-ground-up-rebuild.md §5). It stays the live
// instance-keyed firehose: the flat columns (instance, type, seq, tick, ts)
// cover the indexable / orderable dimensions, the inner payload sits in `data`
// as JSON.
//
// M13 (option a, "match matters more than machine"): an optional `game`
// relation lets the game-end persistence path stamp each event with the game it
// belonged to, so per-game replay/stats can read events by game FK while the
// firehose stays instance-keyed for live capture. The collection is the same;
// only the `game` column + index are added.
//
// Indexes cover three read patterns: "events for instance N since seq S",
// "events of semantic type T in instance N", and "events for game G".
func registerGameEventsCollection(app *pocketbase.PocketBase) error {
	gamesCol, err := app.FindCollectionByNameOrId("games")
	if err != nil {
		return err // games must register first (identity.go phase 4)
	}

	// Reconcile an existing collection: add the `game` relation + index if an
	// older db predates option-a. Dev dbs are ephemeral, but keep prod
	// upgrade-safe rather than silently skipping the new column.
	if existing, err := app.FindCollectionByNameOrId("game_events"); err == nil {
		if existing.Fields.GetByName("game") != nil {
			return nil
		}
		existing.Fields.Add(&core.RelationField{Name: "game", CollectionId: gamesCol.Id, MaxSelect: 1})
		existing.AddIndex("idx_game_events_game", false, "game", "")
		return app.Save(existing)
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
		// M13 option-a: optional back-link to the game this event belonged to,
		// stamped by the game-end persistence path. Empty for live/idle events
		// outside any recorded game.
		&core.RelationField{
			Name:         "game",
			CollectionId: gamesCol.Id,
			MaxSelect:    1,
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
	// Per-game queries: all events stamped with a given game.
	collection.AddIndex("idx_game_events_game", false, "game", "")

	// Admin-only via the dashboard; future read endpoints can surface
	// filtered views. No public REST CRUD.
	collection.ListRule = nil
	collection.ViewRule = nil
	collection.CreateRule = nil
	collection.UpdateRule = nil
	collection.DeleteRule = nil

	return app.Save(collection)
}
