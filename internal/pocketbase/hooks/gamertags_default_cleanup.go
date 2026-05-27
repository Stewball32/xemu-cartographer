package hooks

import (
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerGamertagsDefaultCleanupHook)
}

// registerGamertagsDefaultCleanupHook clears users.default_gamertag before a
// gamertag row is deleted so the relation FK doesn't dangle. We use the
// pre-delete (OnRecordDelete) hook rather than the after-delete variant
// because we want the user.Save() to complete before the gamertag row
// vanishes — if the user save fails, returning the error short-circuits the
// delete (PB respects the chained error from BindFunc handlers).
//
// Falling back from CascadeDelete on the relation: leaving cascade off keeps
// historical rosters readable through the gamertag relation even after the
// owner trims their tag list. Manual FK cleanup here covers the one place
// where the dangling pointer would actually surface in the API surface
// (/api/me's default_gamertag expansion).
func registerGamertagsDefaultCleanupHook(app *pocketbase.PocketBase) {
	app.OnRecordDelete("gamertags").BindFunc(func(e *core.RecordEvent) error {
		tagID := e.Record.Id

		// PB doesn't have a direct "find users where default_gamertag = X"
		// helper, so do it via FindRecordsByFilter. The relation field is
		// stored as a stringified id, so an equality match is correct.
		users, err := e.App.FindRecordsByFilter(
			"users",
			"default_gamertag = {:tagID}",
			"",
			0,
			0,
			map[string]any{"tagID": tagID},
		)
		if err != nil {
			log.Printf("hooks: gamertag_default_cleanup — find users referencing %s: %v", tagID, err)
			return e.Next()
		}

		for _, u := range users {
			u.Set("default_gamertag", "")
			if err := e.App.Save(u); err != nil {
				log.Printf("hooks: gamertag_default_cleanup — clear default on user %s: %v", u.Id, err)
				return err
			}
		}

		return e.Next()
	})
}
