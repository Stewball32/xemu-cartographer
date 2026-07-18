package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// registerSyncPresetsCollection creates the `sync_presets` collection (SPEC
// §4.2) — the "express preset" the LAN-sync client pulls: a named bundle of an
// event's checked-in players' profiles, plus selected games (isos) + apps, with
// a priority ordering and a per-category conflict/prune policy.
//
// Exactly one preset is `active` — the client resolves it by default (GET
// /api/lan/sync/manifest?preset=active). Single-active is NOT yet enforced (see
// the TODO on `active`).
//
// Fields mirror the SPEC table: name, active, event, games, apps, priority
// (json id→int ordering map, higher = fill first), policy (json per-category
// {conflict, prune}). NOTE: unlike the first scaffold, there is NO explicit
// `players` relation and NO separate per-category select fields — players are
// resolved from the event's check-ins, and policy is a single JSON blob (the
// client reads it verbatim into the manifest).
//
// Staleness (SPEC §4.5 / report edge cases): the games/apps relations use
// CascadeDelete=false, so deleting a referenced iso/app NULLIFIES the reference
// (PB drops the id from the list) rather than cascading the preset away — the
// manifest resolver must tolerate dangling ids.
//
// Registered from identity.go (phase 6) AFTER lan_events + isos + apps (its
// relation targets) and after phase 1 (user_roles, for the mutate rule).
func registerSyncPresetsCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "sync_presets") {
		return reconcileSyncPresetsRules(app)
	}

	eventsCol, err := requireCollection(app, "lan_events")
	if err != nil {
		return err
	}
	isosCol, err := requireCollection(app, "isos")
	if err != nil {
		return err // isos must register first (identity.go phase 6, before this)
	}
	appsCol, err := requireCollection(app, "apps")
	if err != nil {
		return err // apps must register first (identity.go phase 6, before this)
	}

	collection := core.NewBaseCollection("sync_presets")
	collection.Fields.Add(
		&core.TextField{Name: "name", Required: true, Min: 1, Max: 120, Presentable: true},
		// The single preset the client pulls by default (?preset=active).
		// TODO(lan-sync): enforce single-active via a hook (clear other presets'
		// `active` on save), or move "active" to a singleton pointer collection.
		&core.BoolField{Name: "active"},
		// The LAN event whose checked-in players' profiles this preset syncs.
		&core.RelationField{
			Name:         "event",
			Required:     true,
			CollectionId: eventsCol.Id,
			MaxSelect:    1,
		},
		// Games (ISOs) to push. preset→iso reference; nullify-on-delete.
		&core.RelationField{
			Name:          "games",
			CollectionId:  isosCol.Id,
			MaxSelect:     999,
			CascadeDelete: false,
		},
		// Apps to push. preset→app reference; nullify-on-delete.
		&core.RelationField{
			Name:          "apps",
			CollectionId:  appsCol.Id,
			MaxSelect:     999,
			CascadeDelete: false,
		},
		// Ordering map: record id → int priority, higher = fill first. Relation
		// lists are unordered sets, so ordering lives here. Shape:
		//   {"iso_ce": 100, "app_xbmc": 50, ...}
		&core.JSONField{Name: "priority", MaxSize: 1 << 16},
		// Per-category conflict/prune policy, read verbatim into the manifest's
		// top-level `policy`. Shape (SPEC §4.3):
		//   {"profiles":{"conflict":"overwrite","prune":true},
		//    "games":{"conflict":"skip","prune":false},
		//    "apps":{"conflict":"skip","prune":false}}
		// conflict ∈ {skip, overwrite}; prune ∈ {true, false}. ("prune" as a
		// per-category conflict verb collapses to overwrite+remove-not-in-set on
		// the client; here it's carried as the policy flag.)
		&core.JSONField{Name: "policy", MaxSize: 1 << 16},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
	)

	collection.AddIndex("idx_sync_presets_active", false, "active", "")

	setSyncPresetsRules(collection)
	return app.Save(collection)
}

func reconcileSyncPresetsRules(app *pocketbase.PocketBase) error {
	existing, err := app.FindCollectionByNameOrId("sync_presets")
	if err != nil {
		return err
	}
	setSyncPresetsRules(existing)
	return app.Save(existing)
}

// setSyncPresetsRules: any authed user may read presets (admin UI + preview);
// only organizers/admins may create/edit/remove them. The on-Xbox client reads
// the RESOLVED manifest via the LAN endpoint (superuser context), not this REST.
func setSyncPresetsRules(c *core.Collection) {
	read := "@request.auth.id != \"\""
	mutate := organizerOrAdmin
	c.ListRule = &read
	c.ViewRule = &read
	c.CreateRule = &mutate
	c.UpdateRule = &mutate
	c.DeleteRule = &mutate
}
