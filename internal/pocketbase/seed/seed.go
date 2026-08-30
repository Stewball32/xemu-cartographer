//go:build dev

package seed

import (
	"fmt"
	"log"
	"os"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/roles"
)

// Run seeds the database with test data.
// This file is only compiled with the "dev" build tag.
func Run(app *pocketbase.PocketBase) error {
	log.Println("Seeding database...")

	// Baseline role rows first — no migration inserts records, so a FRESH dev
	// DB has an empty roles collection and every roles.Grant below (plus the
	// users_default_role hook) would fail. Prod grew its rows live; dev
	// re-mints them each ephemeral boot.
	for _, r := range []struct {
		Slug, Label string
		Level       int
	}{
		{"member", "Member", 0},
		{"organizer", "Organizer", 50},
		{"admin", "Admin", 100},
	} {
		if err := ensureRole(app, r.Slug, r.Label, r.Level); err != nil {
			return fmt.Errorf("seed role %s: %w", r.Slug, err)
		}
	}

	for _, su := range superusers {
		if err := ensureSuperuser(app, su); err != nil {
			return fmt.Errorf("seed superuser %s: %w", su.Email, err)
		}
	}

	for _, u := range users {
		if err := ensureUser(app, u); err != nil {
			return fmt.Errorf("seed user %s: %w", u.Email, err)
		}
	}

	if err := ensureContainersFromSnapshot(app); err != nil {
		return fmt.Errorf("seed containers: %w", err)
	}

	// LAN-sync E2E scenario — opt-in (SEED_LAN_SYNC=true) so it only runs on the
	// dedicated test server, not every `task dev`.
	if os.Getenv("SEED_LAN_SYNC") == "true" {
		if err := SeedLanSync(app); err != nil {
			return fmt.Errorf("seed lan-sync: %w", err)
		}
	}

	log.Println("Seeding complete.")
	return nil
}

func ensureUser(app *pocketbase.PocketBase, u seedUser) error {
	existing, _ := app.FindAuthRecordByEmail("users", u.Email)
	if existing != nil {
		log.Printf("  user %s: exists, skipping", u.Email)
		return nil
	}

	collection, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}

	record := core.NewRecord(collection)
	record.Set("email", u.Email)
	record.Set("password", u.Password)
	record.Set("username", u.Username)
	// M08d: isAdmin column is gone — admin status is a user_roles row
	// pointing at the admin role. Grant happens post-save below.

	if err := app.Save(record); err != nil {
		return err
	}

	if u.IsAdmin {
		if err := roles.Grant(app, record.Id, "admin", nil); err != nil {
			return fmt.Errorf("seed user %s: grant admin role: %w", u.Email, err)
		}
	}
	if u.IsOrganizer {
		if err := roles.Grant(app, record.Id, "organizer", nil); err != nil {
			return fmt.Errorf("seed user %s: grant organizer role: %w", u.Email, err)
		}
	}

	log.Printf("  user %s: created", u.Email)
	return nil
}

// ensureRole upserts one baseline roles row by slug (idempotent).
func ensureRole(app *pocketbase.PocketBase, slug, label string, level int) error {
	if existing, _ := app.FindFirstRecordByData("roles", "slug", slug); existing != nil {
		return nil
	}
	collection, err := app.FindCollectionByNameOrId("roles")
	if err != nil {
		return err
	}
	record := core.NewRecord(collection)
	record.Set("slug", slug)
	record.Set("label", label)
	record.Set("level", level)
	if err := app.Save(record); err != nil {
		return err
	}
	log.Printf("  role %s: created", slug)
	return nil
}

func ensureSuperuser(app *pocketbase.PocketBase, su seedSuperuser) error {
	existing, _ := app.FindAuthRecordByEmail("_superusers", su.Email)
	if existing != nil {
		log.Printf("  superuser %s: exists, skipping", su.Email)
		return nil
	}

	collection, err := app.FindCollectionByNameOrId("_superusers")
	if err != nil {
		return err
	}

	record := core.NewRecord(collection)
	record.Set("email", su.Email)
	record.Set("password", su.Password)

	if err := app.Save(record); err != nil {
		return err
	}

	log.Printf("  superuser %s: created", su.Email)
	return nil
}
