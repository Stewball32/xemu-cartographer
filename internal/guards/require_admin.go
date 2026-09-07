package guards

import (
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb"
)

// RequireAdmin checks that the user may join the admin WS room: the authz
// `room.join` decision on the admin room (G-1), which the admin role's
// `admin.*` scope satisfies and superusers always pass. The deps are read
// from pb.Default() at call time; before boot installs them the answer is
// a fail-closed ErrForbidden.
func RequireAdmin(svc *Services, user *core.Record) error {
	if user == nil {
		return ErrAuthRequired
	}
	if svc == nil || svc.App == nil {
		return ErrForbidden
	}
	d := pb.Default()
	p := pb.PrincipalFromAuth(svc.App, d, user)
	if !authz.Can(d, p, authz.ActionRoomJoin, authz.RoomRes(authz.Room{Type: "admin"})) {
		return ErrForbidden
	}
	return nil
}
