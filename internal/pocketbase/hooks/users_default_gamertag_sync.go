package hooks

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerUsersDefaultGamertagSyncHook)
}

// registerUsersDefaultGamertagSyncHook keeps users.gamertag (the in-game name
// written into the signed CE/H2 profile saves) in lockstep with the user's
// DEFAULT gamertag: "your default is the name the profiles carry — changing it
// regenerates both saves" (settings redesign, Stream tab).
//
// The sync happens PRE-persist inside the same update, so the
// users_gamertag_regen hook (registered after this one — Go inits package
// files in name order, and _sync < gamertag_regen) sees the gamertag change
// on the same event and re-saves the profiles against the new name.
//
// A cleared default leaves users.gamertag alone (existing saves keep their
// name rather than being blanked), and a default pointing at a blocked or
// missing row is ignored — a blocked name must not reach the generated saves.
func registerUsersDefaultGamertagSyncHook(app *pocketbase.PocketBase) {
	app.OnRecordUpdate("users").BindFunc(syncGamertagFromDefault)
}

// syncGamertagFromDefault is the handler, exposed as a named function for the
// integration test.
func syncGamertagFromDefault(e *core.RecordEvent) error {
	old := ""
	if orig := e.Record.Original(); orig != nil {
		old = orig.GetString("default_gamertag")
	}
	cur := e.Record.GetString("default_gamertag")
	if cur != "" && cur != old {
		if tag, err := e.App.FindRecordById("gamertags", cur); err == nil && tag != nil {
			if tag.GetString("status") != "blocked" {
				if t := tag.GetString("tag"); t != "" {
					e.Record.Set("gamertag", t)
				}
			}
		}
	}
	return e.Next()
}
