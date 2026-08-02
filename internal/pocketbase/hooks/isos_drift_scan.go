package hooks

import (
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/routine"

	"github.com/Stewball32/xemu-cartographer/internal/isoingest"
	"github.com/Stewball32/xemu-cartographer/internal/lansync"
)

func init() {
	register(registerISOsDriftScanHook)
}

// registerISOsDriftScanHook ensures the ingest dirs exist and re-verifies every
// managed disc against its content-hash anchor on boot. A drifted (tampered /
// truncated) disc is forced unavailable + flagged so it can't boot or sync. Runs
// in a routine.FireAndForget off OnServe so the (potentially re-hashing) scan
// never blocks startup; the cheap size+mtime pre-check means an untampered
// library re-hashes nothing.
func registerISOsDriftScanHook(app *pocketbase.PocketBase) {
	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		cfg := lansync.Load()
		if err := cfg.EnsureDirs(); err != nil {
			log.Printf("isoingest: ensure dirs: %v", err)
		}
		routine.FireAndForget(func() {
			if flagged := isoingest.ScanDrift(app, cfg); len(flagged) > 0 {
				log.Printf("isoingest: boot drift scan flagged %d disc(s): %v", len(flagged), flagged)
			}
		})
		return e.Next()
	})
}
