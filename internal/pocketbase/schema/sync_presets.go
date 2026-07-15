package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// registerSyncPresetsCollection creates the `sync_presets` collection — the
// "express preset" the LAN-sync client pulls: a named bundle of WHICH
// checked-in players' profiles, WHICH games (isos), and WHICH apps to push to a
// station, in priority order, with a per-category conflict policy.
//
// Exactly one preset is the "active" one the client resolves by default (GET
// /api/lan/sync/manifest?preset=active). Single-active is NOT yet enforced —
// see the TODO on the `active` field.
//
// Relations (staleness — see the M/scaffold report's edge-case notes): the
// player/game/app references are PB RelationFields with CascadeDelete=false, so
// deleting a referenced iso/app/user NULLIFIES the reference (PB drops the id
// from the list) rather than cascading the preset away. That keeps a preset
// alive when one of its items disappears; the manifest resolver must therefore
// tolerate dangling/absent ids at read time.
//
// ⚠️ Ordering caveat: PB relation LISTS are unordered sets, so "priority order"
// cannot be expressed by the relation field itself. `priority` holds the
// explicit ordered id sequence the client should process; the resolver reads it
// to order the manifest.
//
// Registered from identity.go (phase 6) AFTER isos + apps (relation targets)
// and after phase 1 (user_roles, for the organizer/admin mutate rule).
func registerSyncPresetsCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "sync_presets") {
		return reconcileSyncPresetsRules(app)
	}

	usersCol, err := requireCollection(app, "users")
	if err != nil {
		return err
	}
	eventsCol, err := requireCollection(app, "game_events")
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
		&core.TextField{Name: "description", Max: 2000},
		// Event this preset draws its checked-in players from (optional; a
		// preset may also pin an explicit player list via `players`). Same
		// game_events scope caveat as checkins.event.
		&core.RelationField{
			Name:         "event",
			CollectionId: eventsCol.Id,
			MaxSelect:    1,
		},
		// Explicit player set (the checked-in players whose CE/H2 profiles sync).
		// Nullify-on-delete. Empty = "resolve from event's checked-in set" (TODO
		// in the manifest resolver).
		&core.RelationField{
			Name:          "players",
			CollectionId:  usersCol.Id,
			MaxSelect:     999,
			CascadeDelete: false,
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
		// Explicit priority ordering (relation lists are unordered). Shape TBD
		// with the client — a stub such as
		//   {"players":["u1",...],"games":["i1",...],"apps":["a1",...]}
		// or a flat ranked list. Resolver reads this to order manifest items.
		&core.JSONField{Name: "priority", MaxSize: 1 << 16},
		// Per-category conflict policy: what the client does when the target
		// already has an item in this category.
		//   skip      — leave the existing item, don't push
		//   overwrite — replace the existing item
		//   prune     — overwrite AND remove target items not in this preset
		&core.SelectField{Name: "profiles_conflict", Values: conflictPolicyValues, MaxSelect: 1},
		&core.SelectField{Name: "games_conflict", Values: conflictPolicyValues, MaxSelect: 1},
		&core.SelectField{Name: "apps_conflict", Values: conflictPolicyValues, MaxSelect: 1},
		// The single preset the client pulls by default. TODO(lan-sync): enforce
		// single-active via a hook (clear other presets' `active` on save), or
		// move "active" to a separate singleton pointer collection.
		&core.BoolField{Name: "active"},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
	)

	collection.AddIndex("idx_sync_presets_active", false, "active", "")

	setSyncPresetsRules(collection)
	return app.Save(collection)
}

// conflictPolicyValues is the shared per-category conflict-policy enum.
var conflictPolicyValues = []string{"skip", "overwrite", "prune"}

func reconcileSyncPresetsRules(app *pocketbase.PocketBase) error {
	existing, err := app.FindCollectionByNameOrId("sync_presets")
	if err != nil {
		return err
	}
	setSyncPresetsRules(existing)
	return app.Save(existing)
}

// setSyncPresetsRules: any authed user may read presets (the admin UI +
// manifest preview); only organizers/admins may create/edit/remove them. The
// on-Xbox client reads the resolved manifest through the LAN endpoint (superuser
// context), not this collection's REST API.
func setSyncPresetsRules(c *core.Collection) {
	read := "@request.auth.id != \"\""
	mutate := organizerOrAdmin
	c.ListRule = &read
	c.ViewRule = &read
	c.CreateRule = &mutate
	c.UpdateRule = &mutate
	c.DeleteRule = &mutate
}
