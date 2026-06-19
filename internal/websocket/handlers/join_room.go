package handlers

import (
	"errors"
	"strings"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/gamertags"
	"github.com/Stewball32/xemu-cartographer/internal/roles"
	"github.com/Stewball32/xemu-cartographer/internal/rostergrace"
	"github.com/Stewball32/xemu-cartographer/internal/websocket/rooms"
)

func init() {
	register("join_room", handleJoinRoom)
}

func handleJoinRoom(e *Event) {
	if e.Room == "" {
		return
	}

	rt, ok := rooms.Resolve(e.Room)
	if !ok {
		e.SendError("not_found", "unknown room type")
		return
	}

	if err := rt.CheckGuards(e.Services, e.User); err != nil {
		e.SendError("forbidden", err.Error())
		return
	}

	// M09 9c: narrow host:* access beyond the room type's RequireAuth guard.
	// The cross-instance summary feed stays admin-only; a per-instance
	// host:<name> room additionally admits a non-admin whose gamertag is in
	// that container's live roster. Non-host rooms (admin:, public:) already
	// passed their own guard list above and are untouched.
	if err := authorizeHostRoom(e); err != nil {
		e.SendError("forbidden", err.Error())
		return
	}

	e.JoinRoom(e.Room)

	// Replay catch-up bytes for the joined room. Only host:* rooms have a
	// scraper-driven replay path; other rooms (admin, public) join silently.
	// rooms.Resolve already checked the type registry, but the "host:" prefix
	// distinguishes between the per-instance and aggregate flavours that
	// share the single host RoomType registration.
	if e.Services == nil || e.Services.Scraper == nil {
		return
	}
	switch {
	case e.Room == rooms.HostAllRoom, e.Room == rooms.SummaryRoom:
		for _, msg := range e.Services.Scraper.JoinReplayForHostAll() {
			e.SendRaw(msg)
		}
	case strings.HasPrefix(e.Room, rooms.HostRoomPrefix+":"):
		// "host:<rest>" where rest is either "<inst>" (legacy all-classes
		// replay) or "<inst>:<class>" (v2 per-class replay).
		rest := e.Room[len(rooms.HostRoomPrefix)+1:]
		parts := strings.SplitN(rest, ":", 2)
		name := parts[0]
		if len(parts) == 1 {
			for _, msg := range e.Services.Scraper.JoinReplayForInstance(name) {
				e.SendRaw(msg)
			}
		} else {
			class := parts[1]
			for _, msg := range e.Services.Scraper.JoinReplayForInstanceClass(name, class) {
				e.SendRaw(msg)
			}
		}
	}
}

// authorizeHostRoom applies the M09 9c access rules to host:* rooms. Returns
// nil when the room isn't a host room (already guarded elsewhere) or the
// caller is admitted, and an error describing the denial otherwise. Fails
// closed: any missing dependency or lookup error denies access.
func authorizeHostRoom(e *Event) error {
	isSummary := e.Room == rooms.HostAllRoom || e.Room == rooms.SummaryRoom
	isInstance := strings.HasPrefix(e.Room, rooms.HostRoomPrefix+":") && !isSummary
	if !isSummary && !isInstance {
		return nil // not a host room — leave it to the room type's guards
	}

	svc := e.Services
	// Admins (superuser or admin role) always get in — any host room,
	// regardless of roster membership.
	if svc != nil && svc.App != nil && roles.IsAdminAuth(svc.App, e.User) {
		return nil
	}

	// The cross-instance dashboard summary is admin-only.
	if isSummary {
		return errors.New("host summary is admin-only")
	}

	// Per-instance room: admit a roster member of that specific instance.
	instance := hostRoomInstance(e.Room)
	if instance == "" || svc == nil || svc.App == nil || svc.Scraper == nil || e.User == nil {
		return errors.New("not in this match")
	}
	tags, err := gamertags.SanitizedForUser(svc.App, e.User.Id)
	if err != nil || len(tags) == 0 {
		return errors.New("not in this match")
	}
	// Grace window (M09): admits a roster member of this instance, and keeps
	// them admitted for the rostergrace TTL after a transient roster drop.
	if rostergrace.Default.Allow(svc.Scraper.Membership(), instance, tags, time.Now()) {
		return nil
	}
	return errors.New("not in this match")
}

// hostRoomInstance extracts the instance name from a "host:<instance>" or
// "host:<instance>:<class>" room name. Returns "" when room has no instance
// segment (e.g. the bare "host" type or a non-host room).
func hostRoomInstance(room string) string {
	prefix := rooms.HostRoomPrefix + ":"
	if !strings.HasPrefix(room, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(room, prefix)
	return strings.SplitN(rest, ":", 2)[0]
}
