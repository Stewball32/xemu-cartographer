package hooks

import (
	"errors"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerAppsImmutableHook)
}

// registerAppsImmutableHook enforces the LAN-sync WRITE-ONCE rule on `apps`: the
// uploaded ZIP (`file`) is immutable after create — replacing the archive means
// delete the row + create a new one (new ID). Metadata (`name`, `description`,
// `title_id`, `available`) and the derived `extracted_*` cache fields stay
// editable. Mirrors the isos immutability guard (isos_immutable.go).
//
// Enforced in a hook, not a PB rule, because rules can't compare a field to its
// previous value.
func registerAppsImmutableHook(app *pocketbase.PocketBase) {
	app.OnRecordUpdate("apps").BindFunc(enforceAppFileImmutable)
}

// errAppFileImmutable is returned when an update tries to replace the app zip.
var errAppFileImmutable = errors.New("apps.file is immutable (write-once) — delete this row and create a new one to upload a different app zip")

func enforceAppFileImmutable(e *core.RecordEvent) error {
	old := e.Record.Original().GetString("file")
	now := e.Record.GetString("file")
	// Original() is empty on create; this hook is update-only. A non-empty old
	// that differs from the new stored file name is a replace attempt.
	if old != "" && old != now {
		return errAppFileImmutable
	}
	return e.Next()
}
