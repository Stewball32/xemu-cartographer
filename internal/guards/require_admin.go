package guards

import (
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/roles"
)

// RequireAdmin checks that the user holds the admin role (via user_roles).
// Superusers also pass. Mirrors the M08 PB-rule subquery shape so PB-side
// and Go-side gating stay in sync.
func RequireAdmin(svc *Services, user *core.Record) error {
	if user == nil {
		return ErrAuthRequired
	}
	if svc == nil || svc.App == nil {
		return ErrForbidden
	}
	if !roles.IsAdminAuth(svc.App, user) {
		return ErrForbidden
	}
	return nil
}
