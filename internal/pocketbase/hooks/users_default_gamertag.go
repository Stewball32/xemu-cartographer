package hooks

import (
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerUsersDefaultGamertagHook)
}

// registerUsersDefaultGamertagHook auto-creates a default gamertag matching
// the user's username on user creation, and points users.default_gamertag at
// it. Saves the user the manual "add my first gamertag" step in /settings/.
//
// Failures are logged but not propagated — the user record itself has
// already been saved by the time this fires, and getting the user logged in
// matters more than the bootstrap convenience. The user can always add a
// gamertag manually later.
func registerUsersDefaultGamertagHook(app *pocketbase.PocketBase) {
	app.OnRecordAfterCreateSuccess("users").BindFunc(func(e *core.RecordEvent) error {
		userID := e.Record.Id
		username := e.Record.GetString("username")
		if username == "" {
			// OAuth signups can land without a username if the provider's
			// mapping didn't populate one. Skip — the user can add a tag
			// manually after picking a username.
			return e.Next()
		}

		gamertagsCol, err := e.App.FindCollectionByNameOrId("gamertags")
		if err != nil {
			log.Printf("hooks: default_gamertag — find gamertags collection: %v", err)
			return e.Next()
		}

		tag := core.NewRecord(gamertagsCol)
		tag.Set("user", userID)
		tag.Set("tag", username)
		if err := e.App.Save(tag); err != nil {
			log.Printf("hooks: default_gamertag — create gamertag for %s: %v", userID, err)
			return e.Next()
		}

		// Re-fetch the user so the update writes the freshest row. The event's
		// Record reference is already in the in-flight transaction; mutating
		// it directly would re-fire the create hook in some PB versions.
		user, err := e.App.FindRecordById("users", userID)
		if err != nil {
			log.Printf("hooks: default_gamertag — re-fetch user %s: %v", userID, err)
			return e.Next()
		}
		user.Set("default_gamertag", tag.Id)
		if err := e.App.Save(user); err != nil {
			log.Printf("hooks: default_gamertag — set user.default_gamertag %s: %v", userID, err)
			return e.Next()
		}

		return e.Next()
	})
}
