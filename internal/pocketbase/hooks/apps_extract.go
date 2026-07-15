package hooks

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/routine"
)

func init() {
	register(registerAppsExtractHook)
}

// registerAppsExtractHook eagerly unpacks an uploaded app ZIP into its derived
// EXTRACTED tree on create, so the LAN-sync client pulls an unpacked app ready
// to drop into the Xbox "Apps" folder. Like the ISO cache, it is keyed to the
// immutable row (the zip is write-once — see apps_immutable), so no content
// hashing is needed, and it is evictable + regenerable.
//
// SCAFFOLD: the actual unzip + cache bookkeeping is stubbed (see extractApp).
func registerAppsExtractHook(app *pocketbase.PocketBase) {
	app.OnRecordAfterCreateSuccess("apps").BindFunc(func(e *core.RecordEvent) error {
		id := e.Record.Id
		file := e.Record.GetString("file")
		routine.FireAndForget(func() { extractApp(app, id, file) })
		return e.Next()
	})
}

// extractApp is the (stubbed) eager unzip step.
//
// TODO(lan-sync): implement:
//  1. Resolve the stored zip on disk (PB file storage path for this row/field)
//     or open it via the filesystem/blob API.
//  2. mkdir a per-row cache dir under LAN_SYNC_EXTRACT_DIR (e.g. <dir>/apps/<id>).
//  3. Unzip into it (archive/zip), guarding against path traversal (zip-slip).
//  4. On success re-fetch the row + set extracted_path / extracted_ready=true /
//     extracted_at=now + app.Save. On failure log + leave extracted_ready=false.
//
// Idempotent + safe to re-run; eviction deletes the cache dir + clears the fields.
func extractApp(app *pocketbase.PocketBase, recordID, file string) {
	_ = app
	_ = recordID
	_ = file
	// no-op stub — see TODO above.
}
