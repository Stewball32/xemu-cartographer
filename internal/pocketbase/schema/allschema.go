package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

var registry []func(app *pocketbase.PocketBase) error

// register adds a schema function to the registry.
// Call this from init() in each domain file.
func register(fn func(app *pocketbase.PocketBase) error) {
	registry = append(registry, fn)
}

// RegisterAll creates or updates all application collections.
// Called from cmd/server/main.go inside the OnServe hook.
func RegisterAll(app *pocketbase.PocketBase) error {
	for _, fn := range registry {
		if err := fn(app); err != nil {
			return err
		}
	}
	return nil
}

// collectionExists returns true if a collection with the given name already exists.
func collectionExists(app *pocketbase.PocketBase, name string) bool {
	_, err := app.FindCollectionByNameOrId(name)
	return err == nil
}

// requireCollection looks up a collection by name and returns it, or an error if not found.
func requireCollection(app *pocketbase.PocketBase, name string) (*core.Collection, error) {
	return app.FindCollectionByNameOrId(name)
}

func strPtr(s string) *string {
	return &s
}

func f64(v float64) *float64 {
	return &v
}
