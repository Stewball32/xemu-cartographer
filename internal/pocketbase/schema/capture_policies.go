package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerCapturePoliciesCollection)
}

// registerCapturePoliciesCollection creates the capture_policies collection.
//
// A row pairs (instance, class) with a capture mode + cadence + optional sink.
// The Manager unions these rows with WS room subscriptions to decide whether
// to perform per-tick reads — see atlas/new_json/04-ground-up-rebuild.md §5.
//
// Resolution is most-specific-wins:
//
//	(instance, class) → (*, class) → (instance, *) → (*, *) → default (auto, no sink)
//
// Both instance and class accept "*" as a wildcard. mode and cadence are
// constrained to known enum values via SelectField; sink is free-form
// (e.g. "pb:game_events" or "file:replays/{game_id}.ndjson"; empty = no sink).
//
// Schema only — the Go resolver, hot-reload hook, and demand wiring land in
// later PRs (14–16).
func registerCapturePoliciesCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "capture_policies") {
		return nil
	}

	collection := core.NewBaseCollection("capture_policies")

	collection.Fields.Add(
		&core.TextField{
			Name:        "instance",
			Required:    true,
			Min:         1,
			Max:         64,
			Presentable: true,
		},
		&core.TextField{
			Name:        "class",
			Required:    true,
			Min:         1,
			Max:         32,
			Presentable: true,
		},
		&core.SelectField{
			Name:      "mode",
			Required:  true,
			MaxSelect: 1,
			Values:    []string{"auto", "always", "never"},
		},
		&core.SelectField{
			Name:      "cadence",
			Required:  true,
			MaxSelect: 1,
			Values:    []string{"default", "engine", "30hz", "10hz", "5hz", "250ms", "500ms", "1s"},
		},
		&core.TextField{
			Name: "sink",
			Max:  256,
		},
		&core.TextField{
			Name: "description",
			Max:  256,
		},
		&core.AutodateField{
			Name:     "created",
			OnCreate: true,
		},
		&core.AutodateField{
			Name:     "updated",
			OnCreate: true,
			OnUpdate: true,
		},
	)

	// Pairwise uniqueness — the resolver assumes at most one row per
	// (instance, class), so the most-specific lookup is deterministic.
	collection.AddIndex("idx_capture_policies_instance_class_unique", true, "instance, class", "")

	// Admin-only — managed via the PB dashboard or a future
	// /api/admin/capture-policies/* surface. No direct REST CRUD.
	collection.ListRule = nil
	collection.ViewRule = nil
	collection.CreateRule = nil
	collection.UpdateRule = nil
	collection.DeleteRule = nil

	return app.Save(collection)
}
