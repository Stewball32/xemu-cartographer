package hooks

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/reservednames"
)

func init() {
	register(registerGamertagsReservedNameHook)
}

// registerGamertagsReservedNameHook checks new + updated gamertags against
// the admin-curated reserved_names pre-list (M22e). A match flips the row's
// status to "pending" so it lands in the /admin/players/ Pending tab for
// admin review instead of going straight to allowed.
//
// Runs on:
//   - OnRecordCreate: every newly-submitted gamertag.
//   - OnRecordUpdate: every tag rename — an owner can't dodge the pre-list
//     by editing an existing tag to a reserved string after creation.
//
// Order: alphabetical init() within the hooks package puts this file before
// `gamertags_status_transitions.go`, so the default-status assignment in the
// transitions hook (status="" → "allowed") sees the pending value already
// set and leaves it alone. Match failures (DB error, missing collection on
// first boot before reserved_names registers) log but don't fail the
// underlying record write — the pre-list is best-effort, not a hard gate.
func registerGamertagsReservedNameHook(app *pocketbase.PocketBase) {
	app.OnRecordCreate("gamertags").BindFunc(func(e *core.RecordEvent) error {
		applyReservedNamePending(e, "tag")
		return e.Next()
	})

	app.OnRecordUpdate("gamertags").BindFunc(func(e *core.RecordEvent) error {
		// Only re-evaluate when the moderated field changed; mechanical updates
		// to other fields (sanitized refresh, etc.) shouldn't reflip status.
		if e.Record.Original().GetString("tag") == e.Record.GetString("tag") {
			return e.Next()
		}
		applyReservedNamePending(e, "tag")
		return e.Next()
	})
}

// applyReservedNamePending is the shared body for the create + update paths.
// Pulled into a helper so the teams hook (with field="name") can call the
// identical pattern. Reads the candidate string off the record by field
// name, runs the reserved-name match, and sets status="pending" on a hit.
func applyReservedNamePending(e *core.RecordEvent, field string) {
	candidate := e.Record.GetString(field)
	matched, pattern, err := reservednames.Match(e.App, candidate)
	if err != nil {
		e.App.Logger().Warn("M22e: reserved-name match failed", "collection", e.Record.Collection().Name, "id", e.Record.Id, "err", err)
		return
	}
	if !matched {
		return
	}
	e.App.Logger().Info("M22e: auto-flagged by reserved-name pre-list", "collection", e.Record.Collection().Name, "candidate", candidate, "pattern", pattern)
	e.Record.Set("status", "pending")
}
