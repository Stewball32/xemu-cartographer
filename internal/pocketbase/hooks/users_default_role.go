package hooks

import (
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/roles"
)

func init() {
	register(registerUsersDefaultRoleHook)
}

// registerUsersDefaultRoleHook auto-grants the `member` role to every newly
// created user. Mirrors the users_default_gamertag hook in spirit: bootstrap
// convenience, best-effort, log + continue on failure (the user is already
// saved by the time this fires; getting them logged in matters more than the
// role bootstrap).
//
// roles.Grant is idempotent, so reboots / retries are safe. actorIs nil here
// — the row records "system" provenance and the audit payload will carry
// ByMigration=true, mirroring the M08 migration backfill.
func registerUsersDefaultRoleHook(app *pocketbase.PocketBase) {
	app.OnRecordAfterCreateSuccess("users").BindFunc(func(e *core.RecordEvent) error {
		if err := roles.Grant(e.App, e.Record.Id, "member", nil); err != nil {
			log.Printf("hooks: users_default_role — grant member to %s: %v", e.Record.Id, err)
		}
		return e.Next()
	})
}
