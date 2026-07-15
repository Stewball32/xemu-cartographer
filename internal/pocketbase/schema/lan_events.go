package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// registerLanEventsCollection creates the `lan_events` collection — the LAN
// meetup/session scope for check-ins (SPEC §4.1 scope note + §6). This is the
// authoritative correction to the earlier scaffold: check-ins are NOT scoped to
// `game_events` (the scraper's per-tick telemetry firehose — wrong semantics),
// they are scoped to a lan_event (a real-world session like "NorCal Summer LAN").
//
// Deliberately small: an organizer creates one per meetup, players check in
// against it, and a sync_preset resolves that event's checked-in players into
// the profile set. Surfaces as the manifest's top-level `event {id, label}`.
//
// Registered from identity.go (phase 6) before checkins + sync_presets (both
// relate to it); its mutate rule composes organizerOrAdmin (phase 1).
func registerLanEventsCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "lan_events") {
		return reconcileLanEventsRules(app)
	}

	collection := core.NewBaseCollection("lan_events")
	collection.Fields.Add(
		// Human label shown in the manifest's `event.label`, e.g. "NorCal Summer LAN".
		&core.TextField{Name: "label", Required: true, Min: 1, Max: 120, Presentable: true},
		&core.TextField{Name: "description", Max: 2000},
		// When the session runs (informational; scope is by relation, not date).
		&core.DateField{Name: "starts_at"},
		// Whether this is the current session organizers are checking players
		// into. Informational — the manifest resolves by preset.event, not by
		// this flag. TODO(lan-sync): optionally enforce single-active like presets.
		&core.BoolField{Name: "active"},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
	)

	collection.AddIndex("idx_lan_events_active", false, "active", "")

	setLanEventsRules(collection)
	return app.Save(collection)
}

func reconcileLanEventsRules(app *pocketbase.PocketBase) error {
	existing, err := app.FindCollectionByNameOrId("lan_events")
	if err != nil {
		return err
	}
	setLanEventsRules(existing)
	return app.Save(existing)
}

// setLanEventsRules: any authed user may read the event list (to pick one to
// check into); only organizers/admins may create/edit/remove sessions.
func setLanEventsRules(c *core.Collection) {
	read := "@request.auth.id != \"\""
	mutate := organizerOrAdmin
	c.ListRule = &read
	c.ViewRule = &read
	c.CreateRule = &mutate
	c.UpdateRule = &mutate
	c.DeleteRule = &mutate
}
