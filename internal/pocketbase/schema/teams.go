package schema

import (
	"fmt"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// registerTeamsCollection creates the teams collection.
//
// Any authenticated user can create a team; the creator is recorded in
// created_by and is auto-rostered as owner+manager by the settings UI when
// the team is born (see sveltekit/src/routes/settings/).
//
// M22d adds 4-state moderation on top of the M7 shape. `status` mirrors the
// gamertags enum (approved / allowed / pending / blocked); transitions are
// audited + auto-downgrade on name change is enforced by the
// teams_status_transitions hook ([internal/pocketbase/hooks/]). Owner update
// + delete rights are gated on status != "blocked" (admin always can).
//
// Update rights are held by created_by plus admin. Captain/manager edit
// rights are deliberately not enforced via the PB rule engine —
// @collection.X subqueries hit a chicken-and-egg with rosters' relation back
// to teams. Delegate that through a custom /api/admin/teams/* route if it
// matters.
func registerTeamsCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "teams") {
		if err := migrateTeamsAddStatus(app); err != nil {
			return err
		}
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
		&core.SelectField{
			Name:   "status",
			Values: teamsStatusValues,
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

// teamsStatusValues is the SelectField allow-list for teams.status. Keep in
// sync with the const block in [internal/pocketbase/hooks/teams_status_transitions.go].
var teamsStatusValues = []string{"approved", "allowed", "pending", "blocked"}

// migrateTeamsAddStatus is the M22d one-shot migration that adds the
// `status` SelectField to an existing teams collection and backfills every
// row to "allowed". Unlike the M22b gamertags migration there's no prior
// `blocked` column to translate — teams never had one — so backfill is just
// "give every empty status the default 'allowed' value". No synthetic
// audit_log rows either (no prior block events to record).
//
// Idempotent: skips entirely once every row has a non-empty status.
func migrateTeamsAddStatus(app *pocketbase.PocketBase) error {
	col, err := app.FindCollectionByNameOrId("teams")
	if err != nil {
		return err
	}

	if col.Fields.GetByName("status") == nil {
		col.Fields.Add(&core.SelectField{
			Name:   "status",
			Values: teamsStatusValues,
		})
		if err := app.Save(col); err != nil {
			return fmt.Errorf("M22d migration: add teams.status field: %w", err)
		}
	}

	rows, err := app.FindAllRecords("teams")
	if err != nil {
		return fmt.Errorf("M22d migration: load teams: %w", err)
	}
	for _, row := range rows {
		if row.GetString("status") != "" {
			continue
		}
		row.Set("status", "allowed")
		if err := app.Save(row); err != nil {
			return fmt.Errorf("M22d migration: backfill team %s: %w", row.Id, err)
		}
	}

	return nil
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
//   - Update:    creator (if status != "blocked") OR admin.
//   - Delete:    creator (if status != "blocked") OR admin.
//
// Captain/manager edit rights are deliberately not enforced via the PB rule
// engine — @collection.X subqueries hit a chicken-and-egg with rosters'
// relation back to teams. Roster moderation flows through the admin
// dashboard for now; if it later turns out that delegating roster edits to
// non-creator captains matters, gate that through a custom route under
// /api/admin/teams/* that does the check in Go (where it has full access to
// guards.Services).
//
// Like gamertags, the PB rules gate *whether* an update can land, not
// *which fields* can be touched — the teams_status_transitions hook enforces
// "owners can't promote themselves to approved" and the auto-downgrade-on-
// rename semantic.
func setTeamsRules(c *core.Collection) {
	listView := "@request.auth.id != \"\""
	create := "@request.auth.id != \"\""
	update := "(created_by = @request.auth.id && status != \"blocked\") || @request.auth.isAdmin = true"
	del := "(created_by = @request.auth.id && status != \"blocked\") || @request.auth.isAdmin = true"

	c.ListRule = &listView
	c.ViewRule = &listView
	c.CreateRule = &create
	c.UpdateRule = &update
	c.DeleteRule = &del
}
