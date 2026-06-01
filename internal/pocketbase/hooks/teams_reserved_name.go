package hooks

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerTeamsReservedNameHook)
}

// registerTeamsReservedNameHook is the teams-side parallel of
// gamertags_reserved_name. Same semantics: a reserved-name match flips the
// new/renamed row to status="pending" so it lands in the /admin/rosters/
// Teams Pending tab for review instead of "allowed". Shares
// applyReservedNamePending with the gamertags hook so the matching logic
// stays in one place.
func registerTeamsReservedNameHook(app *pocketbase.PocketBase) {
	app.OnRecordCreate("teams").BindFunc(func(e *core.RecordEvent) error {
		applyReservedNamePending(e, "name")
		return e.Next()
	})

	app.OnRecordUpdate("teams").BindFunc(func(e *core.RecordEvent) error {
		if e.Record.Original().GetString("name") == e.Record.GetString("name") {
			return e.Next()
		}
		applyReservedNamePending(e, "name")
		return e.Next()
	})
}
