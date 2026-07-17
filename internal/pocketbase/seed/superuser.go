package seed

import (
	"fmt"
	"log"
	"os"

	"github.com/pocketbase/pocketbase/core"
)

// EnsureEnvSuperuser bootstraps a PocketBase superuser from the environment —
// the beta/prod-style path that the dev-only seeder (`Run`, //go:build dev)
// cannot cover. This file has NO build tag, so it compiles into every binary
// and runs regardless of dev/prod.
//
// Behavior (env holds secrets only — NO password is ever hardcoded):
//   - If SEED_SUPERUSER_EMAIL or SEED_SUPERUSER_PASSWORD is unset, it's a no-op.
//   - If a superuser with that email already exists, it's left untouched
//     (idempotent — won't reset a password changed out-of-band, won't duplicate).
//   - Otherwise it creates the superuser.
//
// Called from main.go's OnServe hook. Stewart sets the env vars per tier.
func EnsureEnvSuperuser(app core.App) error {
	email := os.Getenv("SEED_SUPERUSER_EMAIL")
	password := os.Getenv("SEED_SUPERUSER_PASSWORD")
	if email == "" || password == "" {
		return nil // opt-in; unset = no bootstrap
	}

	if existing, _ := app.FindAuthRecordByEmail(core.CollectionNameSuperusers, email); existing != nil {
		log.Printf("superuser %s: exists, skipping bootstrap", email)
		return nil
	}

	col, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		return fmt.Errorf("find superusers collection: %w", err)
	}
	r := core.NewRecord(col)
	r.Set("email", email)
	r.Set("password", password)
	if err := app.Save(r); err != nil {
		return fmt.Errorf("create superuser %s: %w", email, err)
	}
	log.Printf("superuser %s: created from SEED_SUPERUSER_* env", email)
	return nil
}
