package middleware

import (
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// RequireCustomAuth demonstrates manual auth validation for cases
// where you need more control than apis.RequireAuth() provides
// (e.g., conditional anonymous access, custom token claims).
func RequireCustomAuth() func(*core.RequestEvent) error {
	return func(e *core.RequestEvent) error {
		if e.Auth == nil {
			return apis.NewUnauthorizedError("authentication required", nil)
		}
		return e.Next()
	}
}
