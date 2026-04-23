package middleware

import (
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// RequireAdmin returns middleware that checks the authenticated user's
// "isAdmin" field. Returns 403 if the user is not an admin.
// Must run after auth middleware.
func RequireAdmin() func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if e.Auth == nil {
			return apis.NewUnauthorizedError("authentication required", nil)
		}
		if !e.Auth.GetBool("isAdmin") {
			return apis.NewForbiddenError("admin access required", nil)
		}
		return e.Next()
	}
}
