package hooks

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/routine"
)

func init() {
	register(registerISOsExtractHook)
}

// registerISOsExtractHook eagerly produces the derived EXTRACTED tree for an ISO
// the moment its catalog row is created, so headless stations pull an unpacked
// disc rather than a raw XISO. The cache is keyed to the immutable row (the
// binary is write-once — see isos_immutable), so it never needs content hashing;
// it is evictable + regenerable at any time.
//
// SCAFFOLD: the actual extract-xiso shell-out + cache bookkeeping is stubbed
// (see extractISO). Wired on AfterCreateSuccess so the row exists before the
// (potentially slow, multi-GiB) extraction runs; done in a routine.FireAndForget
// so record creation isn't blocked on it.
func registerISOsExtractHook(app *pocketbase.PocketBase) {
	app.OnRecordAfterCreateSuccess("isos").BindFunc(func(e *core.RecordEvent) error {
		// Clone the fields the goroutine needs — event records aren't
		// concurrency-safe (CLAUDE.md convention).
		id := e.Record.Id
		filename := e.Record.GetString("filename")
		routine.FireAndForget(func() { extractISO(app, id, filename) })
		return e.Next()
	})
}

// extractISO is the (stubbed) eager extraction step.
//
// TODO(lan-sync): implement:
//  1. Resolve `filename` against the shared ISO dir (podman Config.ISODir).
//  2. mkdir a per-row cache dir under LAN_SYNC_EXTRACT_DIR (e.g. <dir>/isos/<id>).
//  3. Shell out to extract-xiso (installed at /usr/bin/extract-xiso):
//     `extract-xiso -d <cacheDir> <isoPath>` (or -x), streaming, with a
//     timeout + context cancellation.
//  4. On success, re-fetch the row and set extracted_path / extracted_ready=true
//     / extracted_at=now, then app.Save. On failure, log + leave
//     extracted_ready=false so the manifest advertises the row as not-ready.
//
// Extraction should be idempotent + safe to re-run (eviction just deletes the
// cache dir + clears the fields; this hook — or a future /extract route — rebuilds).
func extractISO(app *pocketbase.PocketBase, recordID, filename string) {
	_ = app
	_ = recordID
	_ = filename
	// no-op stub — see TODO above.
}
