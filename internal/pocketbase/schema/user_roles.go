package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// registerUserRolesCollection creates the user_roles join collection landed
// in M08. One row per (user, role) grant.
//
// `granted_by` is nullable for two reasons: (1) the M08 migration backfills
// historical isAdmin=true users with actor=nil; (2) the users_default_role
// hook auto-creates a `member` row on user-create with no human actor.
//
// CascadeDelete is false on both relations because audit_log rows reference
// the user_roles state in their payload but not as a FK — preserving the
// user_roles row through soft-delete keeps the audit timeline coherent.
// (Hard user delete is not supported per M07d.)
//
// Indexes: unique (user, role) prevents double-grants; lookup (user) backs
// the per-user roles read on every /api/me request and every guard check.
//
// PB rules: list/view = any authed user (role membership isn't sensitive at
// this community's scope — same access level as gamertags + teams). Mutate =
// admin only (initially the pre-M08 isAdmin gate; 8c swaps to the user_roles
// subquery, which means admin can mutate user_roles rows by holding their
// own admin user_roles row — a self-referential bootstrap that needs the
// M08 migration to seed the first admin row).
func registerUserRolesCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "user_roles") {
		return reconcileUserRolesRules(app)
	}

	collection := core.NewBaseCollection("user_roles")

	usersCol, err := requireCollection(app, "users")
	if err != nil {
		return err
	}
	rolesCol, err := requireCollection(app, "roles")
	if err != nil {
		return err
	}

	collection.Fields.Add(
		&core.RelationField{
			Name:          "user",
			Required:      true,
			CollectionId:  usersCol.Id,
			MaxSelect:     1,
			CascadeDelete: false,
		},
		&core.RelationField{
			Name:          "role",
			Required:      true,
			CollectionId:  rolesCol.Id,
			MaxSelect:     1,
			CascadeDelete: false,
		},
		&core.RelationField{
			Name:          "granted_by",
			CollectionId:  usersCol.Id,
			MaxSelect:     1,
			CascadeDelete: false,
		},
		&core.AutodateField{
			Name:     "granted_at",
			OnCreate: true,
		},
	)

	collection.AddIndex("idx_user_roles_user_role_unique", true, "user, role", "")
	collection.AddIndex("idx_user_roles_user", false, "user", "")

	setUserRolesRules(collection)

	return app.Save(collection)
}

func reconcileUserRolesRules(app *pocketbase.PocketBase) error {
	existing, err := app.FindCollectionByNameOrId("user_roles")
	if err != nil {
		return err
	}
	setUserRolesRules(existing)
	return app.Save(existing)
}

// setUserRolesRules: read scoped to the row's owner (a user can see which
// roles they hold via /api/me anyway, so the SDK path stays symmetric);
// mutate is nil because (a) self-referential subquery rules don't work at
// the first-boot save, and (b) admins manage assignments through the M8g
// custom routes which call roles.Grant/Revoke + app.Save (bypasses rules).
//
// To list all assignments (admin UI's Assignments tab) the custom route
// uses app.FindRecords directly — rule-bypassed because the route is
// already gated by RequireAdmin middleware.
func setUserRolesRules(c *core.Collection) {
	ownOnly := "user = @request.auth.id"

	c.ListRule = &ownOnly
	c.ViewRule = &ownOnly
	c.CreateRule = nil
	c.UpdateRule = nil
	c.DeleteRule = nil
}
