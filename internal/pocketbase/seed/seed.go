//go:build dev

package seed

import (
	"fmt"
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
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

	log.Println("Seeding complete.")
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
