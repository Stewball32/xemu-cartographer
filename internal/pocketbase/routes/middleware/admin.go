package middleware

import (
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb"
)

// RequireAdmin returns middleware that admits any principal authz allows the
// given admin route-group action on the global resource: PocketBase
// superusers, users holding a role whose scopes cover the action (the seeded
// admin role carries `admin.*`), or — for `admin.scraper` only — a machine
// key minted with that scope. Unresolvable credentials and anonymous callers
// get 401, everything else that fails the check gets 403.
//
// Each admin route group binds its own action (design §2.3): `/api/admin/*`
// → ActionAdminAdmin, `/api/admin/users/*` → ActionAdminUsers,
// `/api/admin/containers/*` → ActionAdminContainers, `/api/admin/scraper/*`
// → ActionAdminScraper, `/api/admin/xemu/*` → ActionAdminXemu, `/api/pod/*`
// → ActionAdminPod. The deps are read through pb.Default() at request time
// (routes are registered before B-1 installs them in some test setups), and
// a nil default fails closed.
func RequireAdmin(a authz.Action) func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		return pb.Require(pb.Default(), a, nil)(e)
	}
}
