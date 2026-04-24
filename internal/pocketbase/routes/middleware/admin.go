package middleware

import (
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// RequireAdmin returns middleware that admits PocketBase superusers OR any
// users-collection record with isAdmin=true. Returns 403 otherwise.
// Must run after auth middleware.
func RequireAdmin() func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if e.Auth == nil {
			return apis.NewUnauthorizedError("authentication required", nil)
		}
		if e.Auth.IsSuperuser() || e.Auth.GetBool("isAdmin") {
			return e.Next()
		}
		return apis.NewForbiddenError("admin access required", nil)
	}
}
