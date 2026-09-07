//go:build dev

package seed

import (
	"reflect"
	"testing"

	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb/pbtest"
)

// TestSeedRolesMatchAuthz pins the dev seeder's role rows to authz.SeedRoles
// (DESIGN-STEP6 §7.4 B-3): a fresh roles collection ends up with exactly the
// five built-in rows, each carrying the seed's label, level and scopes; rows
// that already exist are left alone (an operator's edited scopes survive a
// reseed); the pass is idempotent; and the authz adapter reads the seeded
// levels back through its roles cache.
func TestSeedRolesMatchAuthz(t *testing.T) {
	app, d := pbtest.NewApp(t)

	// Simulate a hand-wiped dev DB: drop every seeded row except organizer,
	// whose scopes are edited so the "existing rows untouched" branch is
	// exercised too.
	const keep = "organizer"
	customScopes := []string{"library.manage:*"}
	for _, seed := range authz.SeedRoles {
		rec, err := app.FindFirstRecordByData("roles", "slug", seed.Slug)
		if err != nil {
			t.Fatalf("find seeded role %s: %v", seed.Slug, err)
		}
		if seed.Slug == keep {
			rec.Set("scopes", customScopes)
			if err := app.Save(rec); err != nil {
				t.Fatalf("edit role %s: %v", seed.Slug, err)
			}
			continue
		}
		if err := app.Delete(rec); err != nil {
			t.Fatalf("delete role %s: %v", seed.Slug, err)
		}
	}
	d.InvalidateRoles()

	if err := ensureRoles(app); err != nil {
		t.Fatalf("ensureRoles: %v", err)
	}
	d.InvalidateRoles()

	for _, seed := range authz.SeedRoles {
		rec, err := app.FindFirstRecordByData("roles", "slug", seed.Slug)
		if err != nil || rec == nil {
			t.Fatalf("role %s missing after ensureRoles: %v", seed.Slug, err)
		}
		wantScopes := append([]string{}, seed.Scopes...)
		if seed.Slug == keep {
			wantScopes = customScopes
		} else {
			if got := rec.GetString("label"); got != seed.Label {
				t.Errorf("role %s label = %q, want %q", seed.Slug, got, seed.Label)
			}
			if got := rec.GetInt("level"); got != seed.Level {
				t.Errorf("role %s level = %d, want %d", seed.Slug, got, seed.Level)
			}
		}
		if got := scopesOf(t, rec); !reflect.DeepEqual(got, wantScopes) {
			t.Errorf("role %s scopes = %v, want %v", seed.Slug, got, wantScopes)
		}
		// The adapter must see the same level the seed declares (the row is
		// what RoleLevel reads, so a drifted seeder would surface here).
		if seed.Slug != keep {
			if lvl, ok := d.RoleLevel(seed.Slug); !ok || lvl != seed.Level {
				t.Errorf("RoleLevel(%s) = %d,%v, want %d,true", seed.Slug, lvl, ok, seed.Level)
			}
		}
	}

	// Idempotent: a second pass neither errors nor duplicates rows.
	if err := ensureRoles(app); err != nil {
		t.Fatalf("ensureRoles (second pass): %v", err)
	}
	rows, err := app.FindAllRecords("roles")
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}
	if len(rows) != len(authz.SeedRoles) {
		t.Errorf("roles rows = %d, want %d", len(rows), len(authz.SeedRoles))
	}

	// A scopeless role must still store a JSON list, not null, so the roles
	// cache parses it as "no scopes" rather than a missing field (every
	// built-in seed carries a scope now, so exercise ensureRole directly).
	if err := ensureRole(app, "scopeless", "Scopeless", 1, nil); err != nil {
		t.Fatalf("ensureRole(scopeless): %v", err)
	}
	scopeless, err := app.FindFirstRecordByData("roles", "slug", "scopeless")
	if err != nil {
		t.Fatalf("find scopeless: %v", err)
	}
	if raw := scopeless.GetString("scopes"); raw != "[]" {
		t.Errorf("scopeless scopes raw = %q, want \"[]\"", raw)
	}
}

// scopesOf decodes a roles row's scopes JSON list; a null / empty field
// decodes to an empty (non-nil) slice so it compares equal to a seed's
// Scopes: []string{}.
func scopesOf(t *testing.T, rec *core.Record) []string {
	t.Helper()
	var scopes []string
	if err := rec.UnmarshalJSONField("scopes", &scopes); err != nil {
		t.Fatalf("role %s: decode scopes: %v", rec.GetString("slug"), err)
	}
	if scopes == nil {
		scopes = []string{}
	}
	return scopes
}
