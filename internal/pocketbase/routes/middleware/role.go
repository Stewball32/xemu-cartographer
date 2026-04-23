package middleware

import (
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// RequireRole returns middleware that checks the authenticated user's
// "role" field against the required role. Returns 403 if mismatched.
// Must run after auth middleware.
func RequireRole(role string) func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if e.Auth == nil {
			return apis.NewUnauthorizedError("authentication required", nil)
		}
		if e.Auth.GetString("role") != role {
			return apis.NewForbiddenError("insufficient permissions", nil)
		}
		return e.Next()
	}
}
