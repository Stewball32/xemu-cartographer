package hooks

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/routine"

	"github.com/Stewball32/xemu-cartographer/internal/isoingest"
)

func init() {
	register(registerMapsCatalogBackfillHook)
}

// registerMapsCatalogBackfillHook brings the canonical map catalog (`maps`) up
// to date on boot: hashes iso_maps rows minted before the content_hash field
// existed (reading their extracted trees) and mints catalog rows for any
// uncataloged build. FireAndForget off OnServe — hashing a full library of
// caches is seconds, not minutes, but it never belongs on the boot path.
// Steady-state (everything hashed + cataloged) it's a no-op.
func registerMapsCatalogBackfillHook(app *pocketbase.PocketBase) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		routine.FireAndForget(func() {
			isoingest.BackfillCatalog(app)
		})
		return e.Next()
	})
}
