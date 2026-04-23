package guards

import "github.com/pocketbase/pocketbase/core"

// RequireAuth checks that the user is authenticated.
func RequireAuth(svc *Services, user *core.Record) error {
	if user == nil {
		return ErrAuthRequired
	}
	return nil
}
