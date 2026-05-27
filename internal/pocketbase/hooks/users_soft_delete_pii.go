package hooks

import (
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	register(registerUsersSoftDeletePIIHook)
}

// registerUsersSoftDeletePIIHook blanks PII when a user is soft-deleted
// (is_deleted flipped false → true). Keeps the id + username + isAdmin so
// roster history + admin-audit context survive; clears email, name, bio,
// location, avatar so a "deleted" account doesn't leak the person who used
// to own it. Stamps deleted_at to the current UTC time.
//
// Username is intentionally preserved: the users_username_immutable hook
// also blocks changes here, and downstream /u/[username]/ pages need a
// stable handle to look up the tombstoned record and render "[deleted
// user]" — there's no PII in the username itself.
//
// Idempotent: subsequent updates that arrive with is_deleted=true but the
// PII blanks already applied pass through untouched (the false→true
// transition gate skips them).
//
// Pairs with users.AuthRule "is_deleted = false" in schema/users.go — the
// rule blocks login, this hook scrubs the data.
func registerUsersSoftDeletePIIHook(app *pocketbase.PocketBase) {
	app.OnRecordUpdate("users").BindFunc(func(e *core.RecordEvent) error {
		wasDeleted := e.Record.Original().GetBool("is_deleted")
		isDeleted := e.Record.GetBool("is_deleted")
		if wasDeleted || !isDeleted {
			return e.Next()
		}

		// PII blanking. Email becomes a tombstone string keyed off the user
		// id so the column's unique constraint still holds without a real
		// inbox address. Avatar wipes the filename pointer (the file stays
		// on disk; janitor cleanup is out of scope here).
		e.Record.Set("email", "deleted+"+e.Record.Id+"@invalid.local")
		e.Record.Set("emailVisibility", false)
		e.Record.Set("verified", false)
		e.Record.Set("name", "")
		e.Record.Set("bio", "")
		e.Record.Set("location", "")
		e.Record.Set("avatar", "")
		e.Record.Set("default_gamertag", "")

		if deletedAt, err := types.ParseDateTime(time.Now().UTC()); err == nil {
			e.Record.Set("deleted_at", deletedAt)
		}

		return e.Next()
	})
}
