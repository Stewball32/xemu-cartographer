package rooms

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/Stewball32/xemu-cartographer/internal/guards"
)

// HostRoomPrefix is the room-type prefix scraper-related rooms use. Per-instance
// rooms are addressed as "host:<instance-name>"; the cross-instance aggregate
// is addressed as HostAllRoom. Both resolve to this single RoomType because
// rooms.Resolve strips at the first ":" — see registry.go.
const HostRoomPrefix = "host"

// HostAllRoom is the v1 reserved aggregate-room name (legacy). v2 replaces
// it with SummaryRoom; this constant stays during the rebuild window until
// the aggregator switches over.
const HostAllRoom = "host:all"

// SummaryRoom is the v2 cross-instance summary class room. The v2
// aggregator broadcasts here (PR 10); v2 clients subscribe here for the
// dashboard / host-list feed. No instance — the payload is multi-host.
//
// See atlas/new_json/04-ground-up-rebuild.md §2 (`summary` class), §4
// (transport: classes are rooms).
const SummaryRoom = "host:summary"

// reservedInstanceNames is the set of suffixes RoomForInstance refuses
// because they collide with reserved aggregate room names. Both "all"
// (HostAllRoom legacy) and "summary" (SummaryRoom v2) are reserved so a
// pod can't shadow either aggregate feed.
var reservedInstanceNames = map[string]bool{
	"all":     true,
	"summary": true,
}

// scraperClasses is the set of v2 per-instance scraper class names that
// can appear as the third segment in a per-class room name
// ("host:<instance>:<class>"). Mirrors the class table in
// atlas/new_json/04-ground-up-rebuild.md §2.
//
// Excludes "summary" (cross-instance, has its own SummaryRoom), control
// messages ("hello", "error"), and "events_reply" (request-reply, not
// subscribed).
var scraperClasses = map[string]bool{
	"xbox":     true,
	"scenario": true,
	"game":     true,
	// game_filtered is the viewer-facing variant of the game class: identical
	// GamePayload shape, but with the neutral-host dummy + allowlisted dummy
	// gamertags removed server-side (roster.FilterRoster) before broadcast. The
	// raw `game` room stays unfiltered for the debug page; overlays subscribe to
	// this one so dummy filtering stays server-side. See manager broadcastPoll.
	"game_filtered": true,
	"tick":          true,
	"objects":       true,
	"debug":         true,
	"previous_game": true,
	"event":         true,
	// event_filtered is the viewer-facing variant of the event class. Unlike
	// game_filtered it is not shape-identical to its raw sibling: it carries
	// death events only, and its payload type drops victim_pos / killer_pos
	// outright. Deaths involving a hidden dummy are dropped or de-attributed
	// server-side. Overlays subscribe here for the KILLED BY plate; the raw
	// `event` room (positions, every event type) stays for the debug page.
	// See manager/event_filtered.go.
	"event_filtered": true,
}

// ScraperClasses returns the sorted per-instance class names
// RoomForInstanceClass accepts. It exists so the manager's class registry
// (internal/scraper/manager/classes.go) can be pinned against this table in
// both directions from outside the package — the manager imports rooms, so
// that check can only live in an external test.
func ScraperClasses() []string {
	out := make([]string, 0, len(scraperClasses))
	for class := range scraperClasses {
		out = append(out, class)
	}
	sort.Strings(out)
	return out
}

// RoomForInstance is the only sanctioned source of "host:<name>" room names —
// every code path that needs a room name from an instance name routes through
// here so reserved-name and syntax violations are caught at one trust boundary.
//
// Rejects: empty name, the reserved suffixes (HostAllRoom legacy, SummaryRoom
// v2), any ":" (would defeat the prefix:suffix Resolve contract), and any
// whitespace (instance names appear in log lines, .sock filenames, and JSON
// payloads — keep them shell-safe).
func RoomForInstance(name string) (string, error) {
	if err := validateInstanceName(name); err != nil {
		return "", err
	}
	return HostRoomPrefix + ":" + name, nil
}

// RoomForInstanceClass returns the v2 per-(instance, class) room name
// "host:<instance>:<class>". Used by the runner to address per-class
// broadcasts and by clients to subscribe to a specific data class for an
// instance.
//
// Instance validation matches RoomForInstance (non-empty, no reserved
// suffix, no ":" inside, no whitespace). Class must be one of the
// registered scraper classes (scraperClasses).
//
// See atlas/new_json/04-ground-up-rebuild.md §4 (transport: classes are
// rooms).
func RoomForInstanceClass(instance, class string) (string, error) {
	if err := validateInstanceName(instance); err != nil {
		return "", err
	}
	if !scraperClasses[class] {
		return "", fmt.Errorf("rooms: %q is not a known scraper class", class)
	}
	return HostRoomPrefix + ":" + instance + ":" + class, nil
}

// validateInstanceName enforces the input rules shared by RoomForInstance
// and RoomForInstanceClass.
func validateInstanceName(name string) error {
	if name == "" {
		return errors.New("rooms: instance name required")
	}
	if reservedInstanceNames[name] {
		return fmt.Errorf("rooms: %q is reserved (collides with aggregate room)", name)
	}
	if strings.ContainsRune(name, ':') {
		return fmt.Errorf("rooms: instance name %q must not contain ':'", name)
	}
	for _, r := range name {
		if unicode.IsSpace(r) {
			return fmt.Errorf("rooms: instance name %q must not contain whitespace", name)
		}
	}
	return nil
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
