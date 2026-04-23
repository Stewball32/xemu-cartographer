package guards

import "github.com/pocketbase/pocketbase/core"

// GuardFunc checks whether a user is allowed to proceed.
type GuardFunc func(svc *Services, user *core.Record) error
