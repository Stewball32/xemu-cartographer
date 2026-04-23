package guards

import (
	"errors"

	"github.com/pocketbase/pocketbase/core"
)

// RequireInRoom checks that the user is in a specific WebSocket room.
func RequireInRoom(room string) GuardFunc {
	return func(svc *Services, user *core.Record) error {
		if user == nil {
			return ErrAuthRequired
		}
		if svc.WS == nil {
			return errors.New("websocket service not available")
		}
		if !svc.WS.IsInRoom(user.Id, room) {
			return ErrForbidden
		}
		return nil
	}
}
