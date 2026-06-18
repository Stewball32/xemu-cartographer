package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// Registration is coordinated from identity.go (M08): dummy_gamertags' PB
// rules reference @collection.user_roles via hasAdminRole, so it must register
// after user_roles. "dummy_gamertags" sorts before "user_roles" alphabetically,
// so a per-file init() would race the rule validation — hence the identity.go
// chain.

// registerDummyGamertagsCollection creates the M10d global "always-dummy"
// gamertag allowlist. A row names a gamertag that should never appear in
// viewer-facing surfaces (overlays, minimaps, stats) — e.g. a modded
// neutral-host's out-of-bounds dummy player that's keyed to a fixed name.
//
// The roster filter (internal/scraper/roster) consumes the sanitized set of
// these names; the per-container `is_neutral_host` flag (on the containers
// collection) is the other config source, and a per-game manual override
// lands with the M15 stats UI.
func registerDummyGamertagsCollection(app *pocketbase.PocketBase) error {
	// Reconcile-not-skip: refresh rules on an existing collection so a rule
	// tweak here takes effect without wiping pb_data (mirrors capture_policies).
	if existing, err := app.FindCollectionByNameOrId("dummy_gamertags"); err == nil {
		setDummyGamertagsRules(existing)
		return app.Save(existing)
	}

	collection := core.NewBaseCollection("dummy_gamertags")

	collection.Fields.Add(
		&core.TextField{
			Name:        "gamertag",
			Required:    true,
			Min:         1,
			Max:         64,
			Presentable: true,
		},
		&core.TextField{
			Name: "note",
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

	// One row per gamertag string — the allowlist is a set.
	collection.AddIndex("idx_dummy_gamertags_gamertag_unique", true, "gamertag", "")

	setDummyGamertagsRules(collection)

	return app.Save(collection)
}

// setDummyGamertagsRules gates CRUD on the M08 admin role, mirroring
// capture_policies — the allowlist is curated by operators through the PB JS
// SDK, and the roster filter reads it server-side via app.FindRecords (which
// bypasses rules).
func setDummyGamertagsRules(c *core.Collection) {
	adminRule := hasAdminRole
	c.ListRule = &adminRule
	c.ViewRule = &adminRule
	c.CreateRule = &adminRule
	c.UpdateRule = &adminRule
	c.DeleteRule = &adminRule
}
