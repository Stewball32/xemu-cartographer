package guards

import "github.com/pocketbase/pocketbase/core"

// Any returns a guard that passes if at least one sub-guard passes.
func Any(guards ...GuardFunc) GuardFunc {
	return func(svc *Services, user *core.Record) error {
		var lastErr error
		for _, g := range guards {
			if err := g(svc, user); err == nil {
				return nil
			} else {
				lastErr = err
			}
		}
		return lastErr
	}
}

// All returns a guard that passes only if every sub-guard passes.
func All(guards ...GuardFunc) GuardFunc {
	return func(svc *Services, user *core.Record) error {
		for _, g := range guards {
			if err := g(svc, user); err != nil {
				return err
			}
		}
		return nil
	}
}
