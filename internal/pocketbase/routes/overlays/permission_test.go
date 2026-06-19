package overlays

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// setupRoles creates minimal roles + user_roles collections (matching what
// internal/roles.Has queries) and returns helpers to make users + grant roles.
func setupRoles(t *testing.T, app core.App) (mkUser func(email string) *core.Record, grant func(u *core.Record, slug string)) {
	t.Helper()
	usersCol, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatalf("users collection: %v", err)
	}

	rolesCol := core.NewBaseCollection("roles")
	rolesCol.Fields.Add(&core.TextField{Name: "slug", Required: true})
	rolesCol.AddIndex("idx_roles_slug_unique_t", true, "slug", "")
	if err := app.Save(rolesCol); err != nil {
		t.Fatalf("save roles: %v", err)
	}

	urCol := core.NewBaseCollection("user_roles")
	urCol.Fields.Add(
		&core.RelationField{Name: "user", Required: true, CollectionId: usersCol.Id, MaxSelect: 1},
		&core.RelationField{Name: "role", Required: true, CollectionId: rolesCol.Id, MaxSelect: 1},
	)
	if err := app.Save(urCol); err != nil {
		t.Fatalf("save user_roles: %v", err)
	}

	roleID := func(slug string) string {
		r, err := app.FindFirstRecordByData("roles", "slug", slug)
		if err == nil && r != nil {
			return r.Id
		}
		r = core.NewRecord(rolesCol)
		r.Set("slug", slug)
		if err := app.Save(r); err != nil {
			t.Fatalf("save role %q: %v", slug, err)
		}
		return r.Id
	}

	mkUser = func(email string) *core.Record {
		u := core.NewRecord(usersCol)
		u.SetEmail(email)
		u.SetPassword("password12345")
		if err := app.Save(u); err != nil {
			t.Fatalf("save user %q: %v", email, err)
		}
		return u
	}
	grant = func(u *core.Record, slug string) {
		ur := core.NewRecord(urCol)
		ur.Set("user", u.Id)
		ur.Set("role", roleID(slug))
		if err := app.Save(ur); err != nil {
			t.Fatalf("grant %q: %v", slug, err)
		}
	}
	return mkUser, grant
}

func TestCanManageOverlays(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	mkUser, grant := setupRoles(t, app)

	admin := mkUser("admin@test.com")
	grant(admin, "admin")
	overlayMgr := mkUser("helper@test.com")
	grant(overlayMgr, "overlay_manager")
	plain := mkUser("plain@test.com")

	// Superuser record (collection-derived IsSuperuser, no save needed).
	suCol, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		t.Fatalf("superusers collection: %v", err)
	}
	superuser := core.NewRecord(suCol)

	cases := []struct {
		name string
		auth *core.Record
		want bool
	}{
		{"superuser", superuser, true},
		{"admin role", admin, true},
		{"overlay_manager role", overlayMgr, true},
		{"plain user (403)", plain, false},
		{"nil auth", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := canManageOverlays(app, tc.auth); got != tc.want {
				t.Errorf("canManageOverlays(%s) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}
