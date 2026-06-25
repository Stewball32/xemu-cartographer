package hooks

import (
	"log"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerUsersGamertagRegenHook)
}

// registerUsersGamertagRegenHook keeps a user's generated profiles in lockstep
// with their gamertag. users.gamertag is the single source of truth for the
// in-game name; the CE + H2 profile generate hooks read it via the `user`
// relation. So when the gamertag changes, the existing profile save files are
// stale until regenerated — this hook re-saves them (which fires their generate
// hooks against the new name).
//
// Runs the regeneration AFTER the user update persists (post-e.Next), so the
// profile hooks read the new gamertag. Without this, changing the gamertag
// anywhere other than re-saving the profile would leave the on-disk name stale.
func registerUsersGamertagRegenHook(app *pocketbase.PocketBase) {
	app.OnRecordUpdate("users").BindFunc(regenerateProfilesOnGamertagChange)
}

// regenerateProfilesOnGamertagChange is the handler, exposed as a named
// function for the integration test.
func regenerateProfilesOnGamertagChange(e *core.RecordEvent) error {
	old := ""
	if orig := e.Record.Original(); orig != nil {
		old = orig.GetString("gamertag")
	}
	changed := old != e.Record.GetString("gamertag")

	if err := e.Next(); err != nil {
		return err
	}
	if changed {
		regenUserProfiles(e.App, e.Record.Id)
	}
	return nil
}

// regenUserProfiles re-saves the user's CE + H2 profile rows (if any) so their
// generate hooks rebuild the save files against the current gamertag.
// Best-effort: a failure is logged, never fatal to the user update.
func regenUserProfiles(app core.App, userID string) {
	for _, col := range []string{"h2_profiles", "ce_profiles"} {
		rec, err := app.FindFirstRecordByFilter(col, "user = {:u}", dbx.Params{"u": userID})
		if err != nil || rec == nil {
			continue // no profile of this kind yet, or lookup miss
		}
		if err := app.Save(rec); err != nil {
			log.Printf("gamertag regen: re-save %s for user %s: %v", col, userID, err)
		}
	}
}
