package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// registerCheckinsCollection creates the `checkins` collection — the check-in
// layer that drives whose CE/H2 profiles a sync preset pulls. A row means "this
// player is present for this event"; the checked-in set is what sync_presets +
// the manifest resolve against.
//
// A check-in can be created two ways ("player self-mark or organizer-mark"),
// distinguished by `source`:
//   - self:      the player checks themselves in (create rule allows own user)
//   - organizer: an organizer/admin marks a player in
//
// SCOPE / the `event` relation: per Stewart's scaffold note this relates to the
// existing `game_events` collection. ⚠️ NOTE (flagged in the report): today
// `game_events` is the per-TICK live event firehose (kills/medals/…), not a
// tournament/LAN "event/session" grouping. If check-ins should be scoped to a
// LAN session, this target likely wants to become a dedicated `events`
// collection (or reuse `series`). Kept as an OPTIONAL relation for now so the
// scaffold builds and we can reconcile the scope target without a data change.
//
// Registered from identity.go (phase 6) — relates to game_events (phase 4) +
// the built-in users collection, and its create rule composes organizerOrAdmin
// (phase 1 user_roles).
func registerCheckinsCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "checkins") {
		return reconcileCheckinsRules(app)
	}

	usersCol, err := requireCollection(app, "users")
	if err != nil {
		return err
	}
	eventsCol, err := requireCollection(app, "game_events")
	if err != nil {
		return err // game_events must register first (identity.go phase 4)
	}

	collection := core.NewBaseCollection("checkins")
	collection.Fields.Add(
		// The player checked in (the profile owner). Self-mark = own user.
		&core.RelationField{
			Name:          "user",
			Required:      true,
			CollectionId:  usersCol.Id,
			MaxSelect:     1,
			CascadeDelete: true, // a deleted user's check-ins are meaningless
		},
		// Which gamertag the player is checked in AS (a user may hold several).
		// The manifest keys profile sync off this. Text, not a relation, to keep
		// the client + lansaves identity lookup gamertag-string based.
		&core.TextField{Name: "gamertag", Max: 64, Presentable: true},
		// Event scope — see the ⚠️ note above re: game_events vs a LAN session.
		&core.RelationField{
			Name:          "event",
			CollectionId:  eventsCol.Id,
			MaxSelect:     1,
			CascadeDelete: false,
		},
		// Whether the player is currently checked in. Toggling this beats
		// delete/recreate so history + marked_by survive a check-out.
		&core.BoolField{Name: "checked_in"},
		// Who set the state: "self" | "organizer". TODO: enforce that self-marks
		// can only set source="self" for their own user (hook).
		&core.SelectField{
			Name:      "source",
			Values:    []string{"self", "organizer"},
			MaxSelect: 1,
		},
		&core.TextField{Name: "note", Max: 500},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
	)

	// One check-in row per (event, user). Second check-in toggles the existing
	// row rather than creating a duplicate.
	collection.AddIndex("idx_checkins_event_user", true, "event, user", "")
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
// create/update their OWN check-in (self-mark), and organizers/admins may
// mark/unmark anyone. Delete is organizer/admin only.
//
// TODO(lan-sync): tighten — a self-mark should not be able to set
// source="organizer" or edit another player's row. The `||` below currently
// lets a user mutate only rows where user == self, which is the intended
// self-mark path; organizer marks flow through organizerOrAdmin.
func setCheckinsRules(c *core.Collection) {
	read := "@request.auth.id != \"\""
	self := `@request.auth.id = user`
	selfOrOrg := "(" + self + " || " + organizerOrAdmin + ")"
	org := organizerOrAdmin
	c.ListRule = &read
	c.ViewRule = &read
	c.CreateRule = &selfOrOrg
	c.UpdateRule = &selfOrOrg
	c.DeleteRule = &org
}
