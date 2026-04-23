package oauth

import (
	"fmt"
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

var registry []func() (core.OAuth2ProviderConfig, bool)

// register adds a provider factory to the registry.
// Call this from init() in each provider file.
func register(fn func() (core.OAuth2ProviderConfig, bool)) {
	registry = append(registry, fn)
}

// RegisterAll collects all registered OAuth2 provider configs, applies them
// to the users auth collection, and saves once.
// Must be called AFTER schema.RegisterAll (users collection must exist).
func RegisterAll(app *pocketbase.PocketBase) error {
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return fmt.Errorf("oauth.RegisterAll: users collection not found (run schema.RegisterAll first): %w", err)
	}

	// Build a set of previously configured provider names for removal logging.
	existing := make(map[string]bool)
	for _, p := range users.OAuth2.Providers {
		existing[p.Name] = true
	}

	var providers []core.OAuth2ProviderConfig
	for _, fn := range registry {
		cfg, ok := fn()
		if !ok {
			if existing[cfg.Name] {
				log.Printf("oauth: removing previously configured provider %q (env vars missing)\n", cfg.Name)
			} else {
				// log.Printf("oauth: skipping provider %q (env vars missing)\n", cfg.Name)
			}
			continue
		}
		providers = append(providers, cfg)
		log.Printf("oauth: added %q\n", cfg.Name)
	}

	users.OAuth2.Enabled = len(providers) > 0
	users.OAuth2.Providers = providers

	return app.Save(users)
}
