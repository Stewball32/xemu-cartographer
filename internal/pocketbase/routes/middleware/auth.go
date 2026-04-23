package middleware

import (
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
)

// RequireAuth returns PocketBase's built-in auth middleware.
// Use this for standard auth gating on custom routes.
func RequireAuth() *hook.Handler[*core.RequestEvent] {
	return apis.RequireAuth()
}
