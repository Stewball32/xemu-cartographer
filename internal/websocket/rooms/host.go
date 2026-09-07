package rooms

import "github.com/xemu-cartographer/xc-scraper/wire"

// Room naming for scraper rooms is owned by the wire contract package
// (xc-scraper/wire/rooms.go). Everything below is a re-declaration or a thin
// wrapper so existing importers keep compiling; the rules and error strings
// live in wire. This package keeps what is flagship-specific: the RoomType
// registry, Resolve, and guard evaluation (registry.go).

// HostRoomPrefix is the room-type prefix scraper-related rooms use. Per-instance
// rooms are addressed as "host:<instance-name>"; the cross-instance aggregate
// is addressed as HostAllRoom. Both resolve to this single RoomType because
// rooms.Resolve strips at the first ":" — see registry.go.
const HostRoomPrefix = wire.HostRoomPrefix

// HostAllRoom is the v1 reserved aggregate-room name (legacy). v2 replaces
// it with SummaryRoom; this constant stays during the rebuild window until
// the aggregator switches over.
const HostAllRoom = wire.HostAllRoom

// SummaryRoom is the v2 cross-instance summary class room. The v2
// aggregator broadcasts here; v2 clients subscribe here for the
// dashboard / host-list feed. No instance — the payload is multi-host.
//
// See atlas/new_json/04-ground-up-rebuild.md §2 (`summary` class), §4
// (transport: classes are rooms).
const SummaryRoom = wire.SummaryRoom

// scraperClasses is the set of v2 per-instance scraper class names that
// can appear as the third segment in a per-class room name
// ("host:<instance>:<class>"). DERIVED from the wire class registry
// (wire.PerInstanceClasses — every announced class except "summary", which
// has its own SummaryRoom); it is no longer a hand-maintained table.
//
// game_filtered / event_filtered are the viewer-facing variants of game /
// event: the raw rooms stay unfiltered for the debug page, overlays
// subscribe to the filtered ones so dummy filtering stays server-side. See
// manager broadcastPoll and manager/event_filtered.go.
var scraperClasses = func() map[string]bool {
	classes := wire.PerInstanceClasses()
	set := make(map[string]bool, len(classes))
	for _, class := range classes {
		set[class] = true
	}
	return set
}()

// ScraperClasses returns the sorted per-instance class names
// RoomForInstanceClass accepts. It exists so the manager's class registry
// (internal/scraper/manager/classes.go) can be pinned against this table in
// both directions from outside the package — the manager imports rooms, so
// that check can only live in an external test.
func ScraperClasses() []string {
	return wire.ScraperClasses()
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
	return wire.RoomForInstance(name)
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
	return wire.RoomForInstanceClass(instance, class)
}

// ValidateInstanceName enforces the input rules shared by RoomForInstance
// and RoomForInstanceClass. Exported so authz.ParseRoom (which carries a
// copy of these rules for the pure core) can be pinned against it from a
// test — the two must never drift.
func ValidateInstanceName(name string) error {
	return wire.ValidateInstanceName(name)
}

// Clients in any "host:*" room receive scraper broadcasts. Per-instance rooms
// (host:<name>) carry that instance's game-data / tick / event envelopes;
// host:all carries the aggregate summary feed.
//
// Admission is no longer a guard list: the join_room handler calls
// authz.Can(p, "room.join", room) on the parsed room, which folds the old
// RequireAuth guard, the host ladder, and the console door into one check
// (DESIGN-STEP6 §4). Guards stays nil so nothing here can shadow it.
//
// Note: only one RoomType is registered (under "host"). Resolve() at
// registry.go strips at the first ":" before lookup, so both "host:smoke1"
// and "host:all" resolve here. The host:all-vs-host:<name> branching lives
// in the join_room handler and the manager's broadcast paths.
func init() {
	register(&RoomType{
		Name:   HostRoomPrefix,
		Guards: nil,
	})
}
