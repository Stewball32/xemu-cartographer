package resolvers

import (
	"errors"

	"github.com/Stewball32/xemu-cartographer/internal/guards"
)

// GetUserRooms returns the WebSocket rooms a user is currently in.
func GetUserRooms(svc *guards.Services, userID string) ([]string, error) {
	if svc.WS == nil {
		return nil, errors.New("websocket service not available")
	}
	return svc.WS.UserRooms(userID), nil
}

// IsConnected reports whether a user has any active WebSocket connections.
func IsConnected(svc *guards.Services, userID string) (bool, error) {
	if svc.WS == nil {
		return false, errors.New("websocket service not available")
	}
	return svc.WS.IsConnected(userID), nil
}
