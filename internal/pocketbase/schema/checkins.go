package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// registerCheckinsCollection creates the `checkins` collection (SPEC §4.1) — the
// check-in layer that drives whose CE/H2 profiles a sync preset pulls. One row =
// "this player (gamertag) is present for this LAN event".
//
// A check-in is created two ways ("player self-mark or organizer-mark"),
// distinguished by `source` (self | organizer). The checked-in set of a
// lan_event is what sync_presets + the manifest resolve into the profile set.
//
// SCOPE: relates to `lan_events` (the LAN session/meetup), NOT `game_events`.
// The client session correctly flagged `game_events` as the scraper's per-tick
// telemetry stream — wrong semantics — so the scope is the new lan_events
// collection (SPEC §4.1 note + §6).
//
// Fields mirror the SPEC table exactly: event, gamertag, source, created.
//
// Registered from identity.go (phase 6) after lan_events + gamertags (its
// relations) and after phase 1 (user_roles, for the organizer mutate rule).
func registerCheckinsCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "checkins") {
		return reconcileCheckinsRules(app)
	}

	eventsCol, err := requireCollection(app, "lan_events")
	if err != nil {
		return err // lan_events must register first (identity.go phase 6, before this)
	}
	gamertagsCol, err := requireCollection(app, "gamertags")
	if err != nil {
		return err // gamertags registers in phase 3
	}

	collection := core.NewBaseCollection("checkins")
	collection.Fields.Add(
		// The LAN session/meetup this check-in scopes to.
		&core.RelationField{
			Name:          "event",
			Required:      true,
			CollectionId:  eventsCol.Id,
			MaxSelect:     1,
			CascadeDelete: true, // a deleted event's check-ins are meaningless
		},
		// Who is present. Relation → gamertags (the manifest keys profile sync
		// off the gamertag → owning user → ce/h2 profile, mirroring lansaves).
		&core.RelationField{
			Name:          "gamertag",
			Required:      true,
			CollectionId:  gamertagsCol.Id,
			MaxSelect:     1,
			CascadeDelete: true, // a deleted gamertag's check-ins are meaningless
		},
		// Who marked it: "self" (the player) | "organizer".
		&core.SelectField{
			Name:      "source",
			Values:    []string{"self", "organizer"},
			MaxSelect: 1,
		},
		&core.AutodateField{Name: "created", OnCreate: true},
	)

	// One check-in per (event, gamertag) — re-checking is a no-op, not a dupe.
	collection.AddIndex("idx_checkins_event_gamertag", true, "event, gamertag", "")
	collection.AddIndex("idx_checkins_event", false, "event", "")

	setCheckinsRules(collection)
	return app.Save(collection)
}

func reconcileCheckinsRules(app *pocketbase.PocketBase) error {
	existing, err := app.FindCollectionByNameOrId("checkins")
	if err != nil {
		return err
	}
	setCheckinsRules(existing)
	return app.Save(existing)
}

// setCheckinsRules: any authed user may read the check-in list; a player may
// self-mark their OWN gamertag, and organizers/admins may mark/unmark anyone.
//
// The self path uses relation traversal (gamertag.user == auth user), so a
// self-mark can only create/remove a check-in for a gamertag the caller owns.
// TODO(lan-sync): also constrain self-marks to source="self" (a hook), so a
// player can't stamp source="organizer" on their own row.
func setCheckinsRules(c *core.Collection) {
	read := "@request.auth.id != \"\""
	self := `@request.auth.id = gamertag.user`
	selfOrOrg := "(" + self + " || " + organizerOrAdmin + ")"
	c.ListRule = &read
	c.ViewRule = &read
	c.CreateRule = &selfOrOrg
	c.UpdateRule = &selfOrOrg
	c.DeleteRule = &selfOrOrg
}
