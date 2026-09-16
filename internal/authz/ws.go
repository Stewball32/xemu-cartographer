package authz

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// ErrRoomSyntax is returned by ParseRoom for a room name outside the
// registered shapes.
var ErrRoomSyntax = errors.New("authz: room syntax")

// scraperClasses mirrors internal/websocket/rooms.scraperClasses — the v2
// per-instance scraper class names that can appear as the third segment of a
// "host:<instance>:<class>" room. Copied rather than imported so this package
// stays PocketBase-free (rooms imports the guards/services tree); S5 adds the
// drift test TestParseRoomClassesMatchRooms that pins the two lists together.
var scraperClasses = map[string]bool{
	"xbox":           true,
	"scenario":       true,
	"game":           true,
	"game_filtered":  true,
	"tick":           true,
	"objects":        true,
	"debug":          true,
	"previous_game":  true,
	"event":          true,
	"event_filtered": true,
}

// ScraperClasses returns the registered per-instance scraper class names,
// sorted (the S5 drift test compares it against rooms.ScraperClasses()).
func ScraperClasses() []string {
	return sortedKeys(scraperClasses)
}

// ParseRoom parses a WebSocket room name into its shape (design §2.6):
//
//	host:<inst>          bare per-instance room
//	host:<inst>:<class>  per-class room, class ∈ scraperClasses
//	host:all             legacy aggregate
//	host:summary         v2 cross-instance summary
//	<type>               any other registered type ("admin", "public", ...)
//
// Instance names follow rooms.validateInstanceName: non-empty, not a reserved
// aggregate when a class is present, no ':' (implied by the split) and no
// Unicode whitespace. Anything else — "", "host", "host:", four segments, an
// unknown class, "host:all:tick" — wraps ErrRoomSyntax. The parser does not
// know which non-host types are registered; the hub's registry still gets the
// final say on those.
func ParseRoom(name string) (Room, error) {
	if name == "" {
		return Room{}, fmt.Errorf("%w: empty room name", ErrRoomSyntax)
	}
	rm := Room{Raw: name}
	typ, rest, hasRest := strings.Cut(name, ":")
	if typ == "" {
		return Room{}, fmt.Errorf("%w: %q has no type", ErrRoomSyntax, name)
	}
	rm.Type = typ
	if typ != hostRoomType {
		// Non-host rooms are addressed by their type; a suffix is preserved in
		// Raw for the registry but carries no meaning here.
		return rm, nil
	}
	if !hasRest || rest == "" {
		return Room{}, fmt.Errorf("%w: %q needs an instance", ErrRoomSyntax, name)
	}
	segs := strings.Split(rest, ":")
	if len(segs) > 2 {
		return Room{}, fmt.Errorf("%w: %q has too many segments", ErrRoomSyntax, name)
	}
	inst := segs[0]
	if inst == "" {
		return Room{}, fmt.Errorf("%w: %q has an empty instance", ErrRoomSyntax, name)
	}
	if hasSpace(inst) {
		return Room{}, fmt.Errorf("%w: instance %q must not contain whitespace", ErrRoomSyntax, inst)
	}
	rm.Instance = inst
	if len(segs) == 1 {
		return rm, nil
	}
	class := segs[1]
	if hostAggregateInstances[inst] {
		return Room{}, fmt.Errorf("%w: %q — aggregate rooms take no class", ErrRoomSyntax, name)
	}
	if class == "" || !scraperClasses[class] {
		return Room{}, fmt.Errorf("%w: %q is not a known scraper class", ErrRoomSyntax, class)
	}
	rm.Class = class
	return rm, nil
}

// sortedKeys returns the keys of a set, sorted.
func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// hasSpace reports whether s contains any Unicode whitespace.
func hasSpace(s string) bool {
	for _, r := range s {
		if unicode.IsSpace(r) {
			return true
		}
	}
	return false
}

// WS client→server message types (internal/websocket handlers). The
// read-only kinds may only subscribe and request replays; anonymous may only
// subscribe (overlay_readonly_test pins the same split for the console door).
var (
	wsSendReadOnly  = map[string]bool{"join_room": true, "leave_room": true, "request_state": true, "request_events": true}
	wsSendAnonymous = map[string]bool{"join_room": true, "leave_room": true}
)

// WSSendAllowed is the per-kind allow-list for client→server message types
// (design §2.6). pb_user / superuser / internal / machine may send any
// non-empty type (the handler's own guards still apply); spectator / device
// are read-only; anonymous may only join and leave; discord and unknown kinds
// may send nothing.
func WSSendAllowed(k Kind, msgType string) bool {
	if msgType == "" {
		return false
	}
	switch k {
	case KindPBUser, KindSuperuser, KindInternal, KindMachine:
		return true
	case KindSpectator, KindDevice:
		return wsSendReadOnly[msgType]
	case KindAnonymous:
		return wsSendAnonymous[msgType]
	default:
		return false
	}
}

// JoinableInstances narrows the set of live instances a principal may see
// through the WS layer (design §2.6): bound kinds see only their binding (and
// only when it is live — compared under the scope segment fold, like
// predBound, and reported by the live name), users / machines / superusers /
// internal see all, discord sees none. Always returns a non-nil slice that
// the caller owns.
func JoinableInstances(p Principal, all []string) []string {
	switch p.Kind {
	case KindPBUser, KindSuperuser, KindMachine, KindInternal:
		out := make([]string, len(all))
		copy(out, all)
		return out
	case KindSpectator, KindDevice, KindAnonymous:
		bound := p.BoundInstance()
		if bound == "" {
			return []string{}
		}
		for _, name := range all {
			if sameInstance(name, bound) {
				return []string{name}
			}
		}
		return []string{}
	default:
		return []string{}
	}
}
