package rooms

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/Stewball32/xemu-cartographer/internal/guards"
)

// HostRoomPrefix is the room-type prefix scraper-related rooms use. Per-instance
// rooms are addressed as "host:<instance-name>"; the cross-instance aggregate
// is addressed as HostAllRoom. Both resolve to this single RoomType because
// rooms.Resolve strips at the first ":" — see registry.go.
const HostRoomPrefix = "host"

// HostAllRoom is the reserved aggregate-room name. Subscribers receive a
// cross-instance summary feed (one entry per running scraper) rather than any
// one instance's full game-data stream. The reserved suffix "all" is enforced
// by RoomForInstance and (defense-in-depth) by Manager.Start and the discovery
// watcher.
const HostAllRoom = "host:all"

// reservedInstanceName is the single suffix that RoomForInstance refuses.
// Currently just "all" (collides with HostAllRoom). Kept as a const so future
// reservations have an obvious place to land.
const reservedInstanceName = "all"

// RoomForInstance is the only sanctioned source of "host:<name>" room names —
// every code path that needs a room name from an instance name routes through
// here so reserved-name and syntax violations are caught at one trust boundary.
//
// Rejects: empty name, the reserved "all" suffix (collides with HostAllRoom),
// any ":" (would defeat the prefix:suffix Resolve contract), and any whitespace
// (instance names appear in log lines, .sock filenames, and JSON payloads —
// keep them shell-safe).
func RoomForInstance(name string) (string, error) {
	if name == "" {
		return "", errors.New("rooms: instance name required")
	}
	if name == reservedInstanceName {
		return "", fmt.Errorf("rooms: %q is reserved (collides with host:all)", name)
	}
	if strings.ContainsRune(name, ':') {
		return "", fmt.Errorf("rooms: instance name %q must not contain ':'", name)
	}
	for _, r := range name {
		if unicode.IsSpace(r) {
			return "", fmt.Errorf("rooms: instance name %q must not contain whitespace", name)
		}
	}
	return HostRoomPrefix + ":" + name, nil
}

// Clients in any "host:*" room receive scraper broadcasts. Per-instance rooms
// (host:<name>) carry that instance's game-data / tick / event envelopes;
// host:all carries the aggregate summary feed. RequireAuth ensures only
// logged-in PocketBase users can subscribe.
//
// Note: only one RoomType is registered (under "host"). Resolve() at
// registry.go:37-44 strips at the first ":" before lookup, so both
// "host:smoke1" and "host:all" resolve here. The host:all-vs-host:<name>
// branching lives in the join_room handler and the manager's broadcast paths.
func init() {
	register(&RoomType{
		Name:   HostRoomPrefix,
		Guards: []GuardFunc{guards.RequireAuth},
	})
}
