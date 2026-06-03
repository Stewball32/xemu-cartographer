//go:build dev

package seed

import (
	"fmt"
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/roles"
)

// Run seeds the database with test data.
// This file is only compiled with the "dev" build tag.
func Run(app *pocketbase.PocketBase) error {
	log.Println("Seeding database...")

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
	// M08: isAdmin semantics moved from a bool column on users to a
	// user_roles row pointing at the admin role. The migration in
	// schema/users.go backfills pre-M08 admins; for fresh dev seeds we
	// grant directly here so the seeded admin can hit /api/admin/* without
	// a reboot.
	if u.IsAdmin {
		record.Set("isAdmin", true)
	}

	if err := app.Save(record); err != nil {
		return err
	}

	if u.IsAdmin {
		if err := roles.Grant(app, record.Id, "admin", nil); err != nil {
			return fmt.Errorf("seed user %s: grant admin role: %w", u.Email, err)
		}
	}

	log.Printf("  user %s: created", u.Email)
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
