package hooks

import (
	"bytes"
	"fmt"
	"io"
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/routine"
	"github.com/pocketbase/pocketbase/tools/types"

	"github.com/Stewball32/xemu-cartographer/internal/lansync"
)

func init() {
	register(registerAppsExtractHook)
}

// registerAppsExtractHook validates + measures an uploaded app ZIP on create so
// the LAN-sync client knows its on-drive footprint before pulling it. Apps
// download as the stored zip (SPEC §4.4), so — unlike ISOs — there is no tree to
// stream; "extraction" here reads the zip's central directory, computes the
// FATX-rounded UNCOMPRESSED footprint (drive-fill math), and marks the row
// ready. Failures are logged, leaving extracted_ready=false.
func registerAppsExtractHook(app *pocketbase.PocketBase) {
	app.OnRecordAfterCreateSuccess("apps").BindFunc(func(e *core.RecordEvent) error {
		id := e.Record.Id
		routine.FireAndForget(func() {
			if err := ExtractAppRecord(app, id); err != nil {
				log.Printf("lansync: app measure %s: %v", id, err)
			}
		})
		return e.Next()
	})
}

// ExtractAppRecord reads the app's stored zip, computes its footprint, and
// writes the cache fields back (extracted_ready / extracted_at /
// footprint_bytes). Exported + synchronous so the seed shares the hook's path.
func ExtractAppRecord(app core.App, recordID string) error {
	rec, err := app.FindRecordById("apps", recordID)
	if err != nil {
		return err
	}
	// Idempotent: already measured → no-op (avoids the create hook's async fire
	// redoing the seed's synchronous measure). Clear extracted_ready to redo.
	if rec.GetBool("extracted_ready") {
		return nil
	}
	filename := rec.GetString("file")
	if filename == "" {
		return fmt.Errorf("app %s has no uploaded file", recordID)
	}

	data, err := readStoredFile(app, rec, filename)
	if err != nil {
		return err
	}

	cfg := lansync.Load()
	footprint, err := lansync.ZipFootprint(bytes.NewReader(data), int64(len(data)), cfg.FATXCluster)
	if err != nil {
		return err
	}

	rec.Set("extracted_ready", true)
	rec.Set("extracted_at", types.NowDateTime())
	rec.Set("footprint_bytes", footprint)
	return app.Save(rec)
}

// readStoredFile reads a record's stored file field bytes via the PB filesystem.
func readStoredFile(app core.App, rec *core.Record, filename string) ([]byte, error) {
	fsys, err := app.NewFilesystem()
	if err != nil {
		return nil, err
	}
	defer fsys.Close()

	r, err := fsys.GetReader(rec.BaseFilesPath() + "/" + filename)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}
