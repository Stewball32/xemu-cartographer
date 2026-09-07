package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Authz step 1 (design §5.3): assert `user_roles.granted_by` is optional. The
// snapshot already ships it required:false, but a grant made by a PocketBase
// superuser has no `users` id to record, so a DB that drifted to required
// would 500 on every superuser grant. Idempotent: the collection is saved only
// when the flag actually changes. Down is a no-op — nothing here should ever
// be made required again.
func init() {
	m.Register(func(app core.App) error {
		c, err := app.FindCollectionByNameOrId("user_roles")
		if err != nil {
			return err
		}
		f, ok := c.Fields.GetByName("granted_by").(*core.RelationField)
		if !ok || !f.Required {
			return nil
		}
		f.Required = false
		return app.Save(c)
	}, func(app core.App) error {
		return nil
	})
}
