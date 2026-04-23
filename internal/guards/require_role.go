package guards

import "github.com/pocketbase/pocketbase/core"

// RequireRole checks that the user has the specified role.
func RequireRole(role string) GuardFunc {
	return func(svc *Services, user *core.Record) error {
		if user == nil {
			return ErrAuthRequired
		}
		if user.GetString("role") != role {
			return ErrForbidden
		}
		return nil
	}
}
