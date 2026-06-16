package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// Registration is coordinated from identity.go (M08): reserved_names' PB
// rules reference @collection.user_roles, so it must register after
// user_roles.

// registerReservedNamesCollection creates the reserved_names collection
// landed in M22e — an admin-curated list of substring patterns that auto-flag
// matching gamertags + team names as `status="pending"` on create or update,
// instead of letting the default "allowed" through. Per the M22 doc, matches
// flag for review rather than auto-blocking so false positives ("basement"
// contains "ass") still reach admin eyes.
//
// Shape: pattern (text, unique, lowercased+trimmed by the matcher at compare
// time), description (optional admin note explaining why this pattern was
// added), created_by (relation→users), plus auto-dates. Storing the pattern
// case-insensitively at write time would lose the admin's intended casing in
// the UI; the matcher lowercases both sides at compare time instead.
//
// PB rules: list/view authenticated (the reservednames.Match helper needs
// read access from inside record hooks, and admins need to enumerate the list
// in the admin UI); create/update/delete admin-only.
func registerReservedNamesCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "reserved_names") {
		return reconcileReservedNamesRules(app)
	}

	collection := core.NewBaseCollection("reserved_names")

	usersCol, err := requireCollection(app, "users")
	if err != nil {
		return err
	}

	collection.Fields.Add(
		&core.TextField{
			Name:        "pattern",
			Required:    true,
			Min:         1,
			Max:         64,
			Presentable: true,
		},
		&core.TextField{
			Name: "description",
			Max:  300,
		},
		&core.RelationField{
			Name:          "created_by",
			CollectionId:  usersCol.Id,
			MaxSelect:     1,
			CascadeDelete: false,
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

	collection.AddIndex("idx_reserved_names_pattern_unique", true, "pattern", "")

	setReservedNamesRules(collection)

	return app.Save(collection)
}

func reconcileReservedNamesRules(app *pocketbase.PocketBase) error {
	existing, err := app.FindCollectionByNameOrId("reserved_names")
	if err != nil {
		return err
	}
	setReservedNamesRules(existing)
	return app.Save(existing)
}

// setReservedNamesRules:
//   - List/View: any authed user. Tighter than admin-only because the
//     reservednames.Match helper runs inside the gamertag/team create hooks
//     on the auth context of the requester, and rule evaluation needs to let
//     it through. Patterns aren't secret — admins curate them publicly; the
//     list is roughly equivalent to a profanity dictionary.
//   - Create/Update/Delete: admin only.
func setReservedNamesRules(c *core.Collection) {
	listView := "@request.auth.id != \"\""
	mutate := hasAdminRole

	c.ListRule = &listView
	c.ViewRule = &listView
	c.CreateRule = &mutate
	c.UpdateRule = &mutate
	c.DeleteRule = &mutate
}
