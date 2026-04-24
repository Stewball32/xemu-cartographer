package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerUsersCollection)
}

// registerUsersCollection customizes the built-in "users" auth collection
// that PocketBase creates automatically on first boot.
//
// PocketBase ships the users auth collection with these fields already:
//   - id, password, tokenKey, email, emailVisibility, verified (system/auth)
//   - name (text), avatar (file)
//   - created, updated (timestamps)
//
// Don't redefine those here — only add project-specific fields below.
func registerUsersCollection(app *pocketbase.PocketBase) error {
	// No collectionExists() guard — users always exists, so we reconcile on every boot.
	users, err := requireCollection(app, "users")
	if err != nil {
		return err
	}

	// Guard each field with GetByName so reboots stay idempotent.
	if users.Fields.GetByName("username") == nil {
		users.Fields.Add(&core.TextField{
			Name:        "username",
			Min:         2,
			Max:         16,
			Presentable: true,
			Required:    true,
		})
	}

	if users.Fields.GetByName("bio") == nil {
		users.Fields.Add(&core.TextField{
			Name: "bio",
			Max:  500,
		})
	}

	if users.Fields.GetByName("location") == nil {
		users.Fields.Add(&core.TextField{
			Name: "location",
			Max:  100,
		})
	}

	if users.Fields.GetByName("isAdmin") == nil {
		users.Fields.Add(&core.BoolField{
			Name:   "isAdmin",
			Hidden: true,
		})
	}

	// Unique index — idempotent by name
	const idxName = "idx_users_username_unique"
	if users.GetIndex(idxName) == "" {
		users.AddIndex(idxName, true, "username", "")
	}

	users.OAuth2.MappedFields = core.OAuth2KnownFields{
		Name:      "username", // OAuth2 full name  → users.username (user.name adds '#0' behind the username)
		AvatarURL: "avatar",   // OAuth2 avatar URL → users.avatar
		Username:  "username", // OAuth2 username   → users.username
		Id:        "",         // OAuth2 id         → (unmapped)
	}

	return app.Save(users)
}
