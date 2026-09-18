package rooms

import (
	"strings"

	"github.com/xemu-cartographer/xemu-cartographer/internal/guards"
)

// GuardFunc is an alias for guards.GuardFunc so room type files can
// reference it without importing the guards package directly.
type GuardFunc = guards.GuardFunc

// Config holds optional settings for a room type.
type Config struct {
	MaxMembers int // 0 = unlimited.
}

// RoomType defines a category of rooms. Clients join rooms using a
// "type:name" prefix convention (e.g. "admin:dashboard").
//
// Admission is decided by authz.Can(room.join) on the parsed room in the
// join_room handler, not by a guard list: every registered type keeps
// Guards nil and nothing walks it. The field stays so a type file still
// reads as a declaration of "who may enter" — the answer is "see the authz
// rule table" for all of them (DESIGN-STEP6 §4).
type RoomType struct {
	Name   string
	Guards []GuardFunc
	Config Config
}

var registry = map[string]*RoomType{}

// register adds a room type definition. Called from init() in room type files.
func register(rt *RoomType) {
	registry[rt.Name] = rt
}

// Resolve parses a room name like "admin:my-room" and returns
// the matching RoomType and whether it was found.
// A room name without ":" is treated as the type itself.
func Resolve(room string) (*RoomType, bool) {
	prefix := room
	if i := strings.Index(room, ":"); i != -1 {
		prefix = room[:i]
	}
	rt, ok := registry[prefix]
	return rt, ok
}
