package hooks

import (
	"errors"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerISOsImmutableHook)
}

// registerISOsImmutableHook enforces the LAN-sync WRITE-ONCE rule on `isos`: the
// binary a row points to is immutable after create. Concretely, the `filename`
// (which XISO in the shared library this row IS) may not change on update —
// replacing the disc means delete the row + create a new one (new ID), so
// anything caching by row ID knows the bytes are stable. Metadata (`name`,
// `title_id`, `description`, `available`) stays freely editable, as do the
// derived `extracted_*` cache fields.
//
// Enforced in a hook, not a PB rule, because rules can't compare a field to its
// previous value.
func registerISOsImmutableHook(app *pocketbase.PocketBase) {
	app.OnRecordUpdate("isos").BindFunc(enforceISOFilenameImmutable)
}

// errISOFilenameImmutable is returned when an update tries to repoint an ISO row
// at a different library file.
var errISOFilenameImmutable = errors.New("isos.filename is immutable (write-once) — delete this row and create a new one to point at a different disc")

func enforceISOFilenameImmutable(e *core.RecordEvent) error {
	old := e.Record.Original().GetString("filename")
	now := e.Record.GetString("filename")
	// Original() is empty on create; this hook is update-only, so a non-empty
	// old that differs from the new value is a repoint attempt.
	if old != "" && old != now {
		return errISOFilenameImmutable
	}
	return e.Next()
}
