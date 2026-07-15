package hooks

import (
	"log"
	"os"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/routine"
	"github.com/pocketbase/pocketbase/tools/types"

	"github.com/Stewball32/xemu-cartographer/internal/lansync"
)

func init() {
	register(registerISOsExtractHook)
}

// registerISOsExtractHook eagerly produces the derived EXTRACTED tree for an ISO
// when its catalog row is created, so headless stations pull an unpacked disc
// rather than a raw XISO. Runs on AfterCreateSuccess (row exists first) in a
// routine.FireAndForget so the potentially slow, multi-GiB extraction doesn't
// block record creation. Failures are logged, leaving extracted_ready=false so
// the manifest advertises the row as not-ready (the download 409s).
//
// The cache is keyed to the immutable row (isos_immutable), so it never needs
// content hashing and is evictable + regenerable at any time.
func registerISOsExtractHook(app *pocketbase.PocketBase) {
	app.OnRecordAfterCreateSuccess("isos").BindFunc(func(e *core.RecordEvent) error {
		id := e.Record.Id // clone: event records aren't concurrency-safe
		routine.FireAndForget(func() {
			if err := ExtractISORecord(app, id); err != nil {
				log.Printf("lansync: iso extract %s: %v", id, err)
			}
		})
		return e.Next()
	})
}

// ExtractISORecord extracts the ISO for a record id and writes the cache fields
// back onto it (extracted_path / extracted_ready / extracted_at /
// footprint_bytes). Exported + synchronous so the seed (and any future on-demand
// re-extract route) can share the exact path the hook uses. Idempotent.
func ExtractISORecord(app core.App, recordID string) error {
	rec, err := app.FindRecordById("isos", recordID)
	if err != nil {
		return err
	}
	// Idempotent: already-cached (ready + tree present) is a no-op, so the
	// create hook's async fire doesn't redo the seed's synchronous extract. To
	// force a re-extract, clear extracted_ready or delete extracted_path.
	if rec.GetBool("extracted_ready") {
		if p := rec.GetString("extracted_path"); p != "" {
			if _, statErr := os.Stat(p); statErr == nil {
				return nil
			}
		}
	}
	cfg := lansync.Load()
	treeDir, footprint, err := lansync.ExtractISO(cfg, rec.GetString("filename"), recordID)
	if err != nil {
		return err
	}
	rec.Set("extracted_path", treeDir)
	rec.Set("extracted_ready", true)
	rec.Set("extracted_at", types.NowDateTime())
	rec.Set("footprint_bytes", footprint)
	return app.Save(rec)
}
