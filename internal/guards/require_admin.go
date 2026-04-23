package guards

import "github.com/pocketbase/pocketbase/core"

// RequireAdmin checks that the user has the isAdmin flag.
func RequireAdmin(svc *Services, user *core.Record) error {
	if user == nil {
		return ErrAuthRequired
	}
	if !user.GetBool("isAdmin") {
		return ErrForbidden
	}
	return nil
}
