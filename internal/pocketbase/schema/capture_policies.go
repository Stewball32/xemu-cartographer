package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// Registration is coordinated from identity.go (M08): capture_policies' PB
// rules reference @collection.user_roles, so it must register after
// user_roles.

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
	// Reconcile-not-skip: if the collection already exists from a prior
	// boot, refresh its rules (so a rule change in this file takes effect
	// without needing the operator to wipe pb_data). Fields and indexes
	// stay as previously persisted — schema migrations should go through
	// app.MigrationsLoader, but rule tweaks are safe to overwrite.
	if existing, err := app.FindCollectionByNameOrId("capture_policies"); err == nil {
		setCapturePolicyRules(existing)
		return app.Save(existing)
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

	setCapturePolicyRules(collection)

	return app.Save(collection)
}

// setCapturePolicyRules gates CRUD on the M8 admin role so the SvelteKit
// admin page can manage rows via the standard PB JS SDK
// (pb.collection('capture_policies')...). PB superusers bypass these
// rules automatically — the rule eval only runs for the regular users
// auth collection, where it admits the same operator class as the
// RequireAdmin middleware on the /api/admin/* routes.
func setCapturePolicyRules(c *core.Collection) {
	adminRule := hasAdminRole
	c.ListRule = &adminRule
	c.ViewRule = &adminRule
	c.CreateRule = &adminRule
	c.UpdateRule = &adminRule
	c.DeleteRule = &adminRule
}
