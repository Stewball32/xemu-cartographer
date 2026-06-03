package schema

import (
	"fmt"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// registerRolesCollection creates the roles collection landed in M08.
//
// One row per named permission bundle. The slug is the API-stable identifier
// (consumed by RequireRole / guard rules); the label is the human-facing
// display name; level is a coarse rank for "is this user at least an admin?"
// ladder checks (RequireMinLevel(50)).
//
// Baseline rows are seeded inline at first-create — these are required data,
// not dev seed (every deployment needs them for the guard layer to function):
//
//   - superuser / level=100 — nameable slug for future "superuser only" labels.
//     Never granted via user_roles; PB _superusers is a separate auth target,
//     handled by IsSuperuser() checks. The row exists for discoverability.
//   - admin      / level=50  — the M08 successor to users.isAdmin. Every user
//     migrated from isAdmin=true gets a user_roles row pointing here (8b).
//   - member     / level=10  — default role auto-assigned to every new user
//     by the users_default_role hook (8b).
//
// PB rules: list/view = any authed user (role slugs aren't sensitive — every
// page needs them to render). Create/update/delete = admin only (initially the
// pre-M08 isAdmin gate; 8c swaps to the user_roles subquery).
func registerRolesCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "roles") {
		if err := reconcileRolesRules(app); err != nil {
			return err
		}
		return seedBaselineRoles(app)
	}

	collection := core.NewBaseCollection("roles")

	collection.Fields.Add(
		&core.TextField{
			Name:        "slug",
			Required:    true,
			Min:         2,
			Max:         60,
			Pattern:     `^[a-z0-9]+(?:_[a-z0-9]+)*$`,
			Presentable: true,
		},
		&core.TextField{
			Name:     "label",
			Required: true,
			Min:      1,
			Max:      80,
		},
		&core.TextField{
			Name: "description",
			Max:  500,
		},
		&core.NumberField{
			Name:    "level",
			OnlyInt: true,
			Min:     f64(0),
			Max:     f64(100),
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

	collection.AddIndex("idx_roles_slug_unique", true, "slug", "")

	setRolesRules(collection)

	if err := app.Save(collection); err != nil {
		return err
	}

	return seedBaselineRoles(app)
}

func reconcileRolesRules(app *pocketbase.PocketBase) error {
	existing, err := app.FindCollectionByNameOrId("roles")
	if err != nil {
		return err
	}
	setRolesRules(existing)
	return app.Save(existing)
}

// setRolesRules is the M08a admit-by-isAdmin shape. 8c swaps the mutate rules
// to the user_roles subquery shape; the read rules stay open to any authed
// user.
func setRolesRules(c *core.Collection) {
	read := "@request.auth.id != \"\""
	mutate := "@request.auth.isAdmin = true"

	c.ListRule = &read
	c.ViewRule = &read
	c.CreateRule = &mutate
	c.UpdateRule = &mutate
	c.DeleteRule = &mutate
}

// baselineRoles is the M08 required-data set. Idempotent: seedBaselineRoles
// upserts by slug, so adding a new entry here will materialize on next boot
// without breaking existing deployments.
var baselineRoles = []struct {
	Slug, Label, Description string
	Level                    float64
}{
	{Slug: "superuser", Label: "Superuser", Description: "PocketBase _superusers tier. Never granted via user_roles; the row exists for discoverability.", Level: 100},
	{Slug: "admin", Label: "Admin", Description: "Full moderation access (gamertags, teams, reserved names, audit log, scraper, containers, xemu diagnostics).", Level: 50},
	{Slug: "member", Label: "Member", Description: "Default role for every authenticated user.", Level: 10},
}

func seedBaselineRoles(app *pocketbase.PocketBase) error {
	col, err := app.FindCollectionByNameOrId("roles")
	if err != nil {
		return err
	}
	for _, r := range baselineRoles {
		existing, _ := app.FindFirstRecordByData("roles", "slug", r.Slug)
		if existing != nil {
			continue
		}
		row := core.NewRecord(col)
		row.Set("slug", r.Slug)
		row.Set("label", r.Label)
		row.Set("description", r.Description)
		row.Set("level", r.Level)
		if err := app.Save(row); err != nil {
			return fmt.Errorf("seed baseline role %q: %w", r.Slug, err)
		}
	}
	return nil
}
