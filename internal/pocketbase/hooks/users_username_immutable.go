package hooks

import (
	"errors"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerUsersUsernameImmutableHook)
}

// registerUsersUsernameImmutableHook rejects any update to users.username
// after creation. Usernames are the stable handle that owns gamertags +
// rosters; letting them mutate would invite confusion on profile pages and
// break the "gamertags as the change-of-handle mechanism" pattern.
//
// Implemented as a hook rather than a collection UpdateRule because PB rules
// see `@request.body` only for fields actually sent; an empty payload would
// match `@request.body.username = username` only when the field is sent
// AND unchanged, which is the wrong shape for "block changes, allow no-ops".
//
// The hook fires for every update path — API, superuser admin UI, and
// programmatic core.App.Save(). A true rename (legal/incident response)
// requires bypassing the hook with a direct SQL update.
func registerUsersUsernameImmutableHook(app *pocketbase.PocketBase) {
	app.OnRecordUpdate("users").BindFunc(func(e *core.RecordEvent) error {
		original := e.Record.Original()
		oldName := original.GetString("username")
		newName := e.Record.GetString("username")
		if oldName == "" || oldName == newName {
			return e.Next()
		}
		return errors.New("username is immutable; create a new gamertag to change your displayed handle")
	})
}
