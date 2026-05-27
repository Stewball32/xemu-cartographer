package hooks

import (
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerGamertagsSanitizeHook)
}

// registerGamertagsSanitizeHook keeps gamertags.sanitized in lockstep with
// gamertags.tag — lowercased + whitespace-trimmed — so the (user, sanitized)
// unique index can catch case variants ("Stewie" vs "STEWIE") and the
// scraper-side player-name lookup that lands in M9+ can match without a
// per-query LOWER(...) scan.
//
// Fires on both pre-create and pre-update so the field stays accurate as
// admins (or the owner) edit the displayed tag.
func registerGamertagsSanitizeHook(app *pocketbase.PocketBase) {
	app.OnRecordCreate("gamertags").BindFunc(func(e *core.RecordEvent) error {
		e.Record.Set("sanitized", sanitizeGamertag(e.Record.GetString("tag")))
		return e.Next()
	})

	app.OnRecordUpdate("gamertags").BindFunc(func(e *core.RecordEvent) error {
		e.Record.Set("sanitized", sanitizeGamertag(e.Record.GetString("tag")))
		return e.Next()
	})
}

func sanitizeGamertag(tag string) string {
	return strings.ToLower(strings.TrimSpace(tag))
}
