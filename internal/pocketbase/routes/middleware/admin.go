package middleware

import (
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/roles"
)

// RequireAdmin returns middleware that admits PocketBase superusers OR any
// users-collection record holding a user_roles row pointing at the admin
// role. Returns 403 otherwise. Must run after auth middleware.
//
// M08: swapped from `e.Auth.GetBool("isAdmin")` to `roles.IsAdminAuth` —
// callers (admin, pod, scraper, containers, xemu route groups) keep their
// existing RequireAdmin() registrations unchanged.
func RequireAdmin() func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if e.Auth == nil {
			return apis.NewUnauthorizedError("authentication required", nil)
		}
		if roles.IsAdminAuth(e.App, e.Auth) {
			return e.Next()
		}
		return apis.NewForbiddenError("admin access required", nil)
	}
}
