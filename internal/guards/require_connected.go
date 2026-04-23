package guards

import (
	"errors"

	"github.com/pocketbase/pocketbase/core"
)

// RequireConnected checks that the user has an active WebSocket connection.
func RequireConnected(svc *Services, user *core.Record) error {
	if user == nil {
		return ErrAuthRequired
	}
	if svc.WS == nil {
		return errors.New("websocket service not available")
	}
	if !svc.WS.IsConnected(user.Id) {
		return ErrForbidden
	}
	return nil
}
