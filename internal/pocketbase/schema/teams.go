package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// registerTeamsCollection creates the teams collection.
//
// Any authenticated user can create a team; the creator is recorded in
// created_by and is auto-rostered as captain+manager by the settings UI when
// the team is born (see sveltekit/src/routes/settings/).
//
// Update rights are held by created_by plus any current captain/manager
// (resolved via the rosters join in the API rule). Delete is gated on
// created_by + admin only — rosters reference teams with CascadeDelete=false
// so historical roster rows survive if a team is later wound down.
func registerTeamsCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "teams") {
		return reconcileTeamsRules(app)
	}

	collection := core.NewBaseCollection("teams")

	usersCol, err := requireCollection(app, "users")
	if err != nil {
		return err
	}

	collection.Fields.Add(
		&core.TextField{
			Name:        "name",
			Required:    true,
			Min:         2,
			Max:         60,
			Presentable: true,
		},
		&core.TextField{
			Name:     "slug",
			Required: true,
			Min:      2,
			Max:      60,
			Pattern:  `^[a-z0-9]+(?:-[a-z0-9]+)*$`,
		},
		&core.RelationField{
			Name:          "created_by",
			Required:      true,
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

	collection.AddIndex("idx_teams_slug_unique", true, "slug", "")

	setTeamsRules(collection)

	return app.Save(collection)
}

func reconcileTeamsRules(app *pocketbase.PocketBase) error {
	existing, err := app.FindCollectionByNameOrId("teams")
	if err != nil {
		return err
	}
	setTeamsRules(existing)
	return app.Save(existing)
}

// setTeamsRules:
//   - List/View: any authed user.
//   - Create:    any authed user.
//   - Update:    creator OR admin.
//   - Delete:    creator OR admin.
//
// Captain/manager edit rights are deliberately not enforced via the PB rule
// engine — @collection.X subqueries hit a chicken-and-egg with rosters'
// relation back to teams. Roster moderation flows through the admin
// dashboard for now; if it later turns out that delegating roster edits to
// non-creator captains matters, gate that through a custom route under
// /api/admin/teams/* that does the check in Go (where it has full access to
// guards.Services).
func setTeamsRules(c *core.Collection) {
	listView := "@request.auth.id != \"\""
	create := "@request.auth.id != \"\""
	update := "created_by = @request.auth.id || @request.auth.isAdmin = true"
	del := "created_by = @request.auth.id || @request.auth.isAdmin = true"

	c.ListRule = &listView
	c.ViewRule = &listView
	c.CreateRule = &create
	c.UpdateRule = &update
	c.DeleteRule = &del
}
