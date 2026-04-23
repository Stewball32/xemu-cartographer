package rooms

import (
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"github.com/youruser/yourproject/internal/guards"
)

// GuardFunc is an alias for guards.GuardFunc so room type files can
// reference it without importing the guards package directly.
type GuardFunc = guards.GuardFunc

// Config holds optional settings for a room type.
type Config struct {
	MaxMembers int // 0 = unlimited.
}

// RoomType defines a category of rooms with shared access guards.
// Clients join rooms using a "type:name" prefix convention (e.g. "admin:dashboard").
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

// CheckGuards runs all guards for the room type. Returns first error or nil.
func (rt *RoomType) CheckGuards(svc *guards.Services, user *core.Record) error {
	for _, g := range rt.Guards {
		if err := g(svc, user); err != nil {
			return err
		}
	}
	return nil
}
