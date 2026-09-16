package handlers

import (
	"slices"
	"strings"
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/authz/authztest"
	"github.com/Stewball32/xemu-cartographer/internal/websocket/rooms"
)

// TestParseRoomMirrorsValidateInstanceName pins authz.ParseRoom (the pure
// core's copy of the instance-name rules) to rooms.ValidateInstanceName
// (the room-naming chokepoint) on one corpus: a name the registry accepts
// must parse back out of both its bare and its per-class room as that
// instance, and a name it refuses must not parse as an instance in either
// ("host:all" / "host:summary" parse — as the aggregates, never as an
// instance called all). The last two entries are valid today — there is
// no case or length rule — and must stay valid.
func TestParseRoomMirrorsValidateInstanceName(t *testing.T) {
	corpus := []string{
		"",
		"all",
		"summary",
		"a:b",
		"a b",
		"a\tb",
		"a b", // NBSP: unicode.IsSpace, not ASCII
		"pod-a",
		"MiXeD",
		strings.Repeat("x", 64),
	}
	for _, name := range corpus {
		wantOK := rooms.ValidateInstanceName(name) == nil

		bare, bareErr := authz.ParseRoom("host:" + name)
		if got := bareErr == nil && !bare.IsHostAggregate(); got != wantOK {
			t.Errorf("ParseRoom(%q) instance ok=%v, want %v (ValidateInstanceName)", "host:"+name, got, wantOK)
		}
		class, classErr := authz.ParseRoom("host:" + name + ":tick")
		if got := classErr == nil; got != wantOK {
			t.Errorf("ParseRoom(%q) ok=%v, want %v (ValidateInstanceName)", "host:"+name+":tick", got, wantOK)
		}
		if !wantOK {
			continue
		}
		if bare.Instance != name || bare.Class != "" || !bare.IsHost() || bare.IsHostAggregate() {
			t.Errorf("ParseRoom(%q) = %+v, want bare host room for %q", "host:"+name, bare, name)
		}
		if class.Instance != name || class.Class != "tick" {
			t.Errorf("ParseRoom(%q) = %+v, want instance %q class tick", "host:"+name+":tick", class, name)
		}
		// And the registry's own spelling round-trips through the parser.
		roomName, err := rooms.RoomForInstance(name)
		if err != nil {
			t.Fatalf("RoomForInstance(%q): %v", name, err)
		}
		if parsed, err := authz.ParseRoom(roomName); err != nil || parsed.Selector() != roomName {
			t.Errorf("ParseRoom(RoomForInstance(%q)) = %+v, %v; want selector %q", name, parsed, err, roomName)
		}
	}
}

// TestParseRoomClassesMatchRooms is the drift test between the per-class
// table authz hard-codes (so the pure core needs no rooms import) and the
// rooms package's ScraperClasses: the two must be identical, and every
// class must be accepted by both RoomForInstanceClass and ParseRoom.
func TestParseRoomClassesMatchRooms(t *testing.T) {
	core, table := authz.ScraperClasses(), rooms.ScraperClasses()
	if !slices.Equal(core, table) {
		t.Fatalf("authz.ScraperClasses and rooms.ScraperClasses have drifted:\n  authz = %v\n  rooms = %v", core, table)
	}
	for _, class := range table {
		name, err := rooms.RoomForInstanceClass("alpha", class)
		if err != nil {
			t.Errorf("RoomForInstanceClass(alpha, %q): %v", class, err)
			continue
		}
		room, err := authz.ParseRoom(name)
		if err != nil || room.Instance != "alpha" || room.Class != class {
			t.Errorf("ParseRoom(%q) = %+v, %v; want instance alpha class %q", name, room, err, class)
		}
	}
	if _, err := authz.ParseRoom("host:alpha:bogus"); err == nil {
		t.Error("ParseRoom(host:alpha:bogus) accepted an unknown class")
	}
}

// joinCase is one TestJoinRoom_Can_* row: a principal asking for a room,
// and whether the full handler admits it.
type joinCase struct {
	name string
	p    authz.Principal
	room string
	want bool
}

// runJoinCases drives handleJoinRoom for every row against deps and checks
// the outcome: an admitted principal is joined to exactly the room it
// asked for with no error; a refused one gets "forbidden" and no join.
func runJoinCases(t *testing.T, deps authz.Deps, cases []joinCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := runJoinRoom(&Event{Room: tc.room, Authz: deps, Principal: tc.p})
			switch {
			case tc.want && out.joined != tc.room:
				t.Fatalf("%s join %q: joined=%q errCode=%q errMsg=%q, want joined",
					tc.p.Kind, tc.room, out.joined, out.errCode, out.errMsg)
			case tc.want && out.errCode != "":
				t.Fatalf("%s join %q: joined but also sent error %q", tc.p.Kind, tc.room, out.errCode)
			case !tc.want && out.joined != "":
				t.Fatalf("%s join %q: joined=%q, want forbidden", tc.p.Kind, tc.room, out.joined)
			case !tc.want && out.errCode != "forbidden":
				t.Fatalf("%s join %q: errCode=%q, want forbidden", tc.p.Kind, tc.room, out.errCode)
			}
		})
	}
}

func pbUser(id string, scopes ...string) authz.Principal {
	return authz.Principal{Kind: authz.KindPBUser, ID: id, UserID: id, Collection: "users", Scopes: authz.CanonScopes(scopes)}
}

func machine(kid string, scopes ...string) authz.Principal {
	return authz.Principal{Kind: authz.KindMachine, ID: kid, Scopes: authz.CanonScopes(scopes)}
}

func boundKey(kind authz.Kind, kid, instance string, scopes ...string) authz.Principal {
	return authz.Principal{
		Kind:   kind,
		ID:     kid,
		Scopes: authz.CanonScopes(scopes),
		Bound:  map[string]string{"instance": instance},
	}
}

// TestJoinRoom_Can_pb_user_host covers the member ladder on host rooms: a
// rostered gamertag (or owning the box) opens the bare room and every
// class room of that instance and nothing else; a room.join scope opens a
// room without any roster; the aggregate feeds need the scope.
func TestJoinRoom_Can_pb_user_host(t *testing.T) {
	deps := &authztest.FakeDeps{
		Rostered: map[string]bool{authztest.Key("u1", "box1"): true},
		Owned:    map[string]string{"u2": "box2"},
	}
	rostered, owner, plain := pbUser("u1"), pbUser("u2"), pbUser("u3")
	scoped := pbUser("u4", "room.join:host:box9:tick", "room.join:host:summary")
	runJoinCases(t, deps, []joinCase{
		{"rostered bare", rostered, "host:box1", true},
		{"rostered objects", rostered, "host:box1:objects", true},
		{"rostered other instance", rostered, "host:box2", false},
		{"rostered other class room", rostered, "host:box2:tick", false},
		{"rostered summary", rostered, "host:summary", false},
		{"rostered legacy all", rostered, "host:all", false},
		{"box owner bare", owner, "host:box2", true},
		{"box owner game", owner, "host:box2:game", true},
		{"box owner other instance", owner, "host:box1", false},
		{"plain user bare", plain, "host:box1", false},
		{"plain user class", plain, "host:box1:game_filtered", false},
		{"scoped class room", scoped, "host:box9:tick", true},
		{"scoped class room other class", scoped, "host:box9:game", false},
		{"scoped summary", scoped, "host:summary", true},
		{"scoped legacy all", scoped, "host:all", false},
	})
}

// TestJoinRoom_Can_machine_host: a machine key is scope-only on host rooms
// and may never take a bare per-instance room (A.10 — those carry the
// unfiltered legacy replay and are pb_user only).
func TestJoinRoom_Can_machine_host(t *testing.T) {
	deps := &authztest.FakeDeps{}
	wide := machine("k1", "room.join:*")
	narrow := machine("k2", "room.join:host:box1:tick")
	none := machine("k3")
	runJoinCases(t, deps, []joinCase{
		{"wildcard class room", wide, "host:box1:tick", true},
		{"wildcard summary", wide, "host:summary", true},
		{"wildcard legacy all", wide, "host:all", true},
		{"wildcard bare room denied", wide, "host:box1", false},
		{"narrow matching class", narrow, "host:box1:tick", true},
		{"narrow other class", narrow, "host:box1:game", false},
		{"narrow other instance", narrow, "host:box2:tick", false},
		{"narrow summary", narrow, "host:summary", false},
		{"no scopes class room", none, "host:box1:tick", false},
	})
}

// TestJoinRoom_Can_spectator_host and its device twin: a bound key needs
// both a scope naming the class and the binding to match — a scope for
// another instance, or a binding elsewhere, opens nothing; bare rooms and
// the aggregates are never theirs.
func TestJoinRoom_Can_spectator_host(t *testing.T) {
	deps := &authztest.FakeDeps{}
	bound := boundKey(authz.KindSpectator, "s1", "box1", "room.join:host:box1:*")
	wideScope := boundKey(authz.KindSpectator, "s2", "box1", "room.join:*")
	unbound := boundKey(authz.KindSpectator, "s3", "", "room.join:*")
	wrongBinding := boundKey(authz.KindSpectator, "s4", "box2", "room.join:host:box1:*")
	runJoinCases(t, deps, []joinCase{
		{"bound tick", bound, "host:box1:tick", true},
		{"bound objects", bound, "host:box1:objects", true},
		{"bound other instance", bound, "host:box2:tick", false},
		{"bound bare denied", bound, "host:box1", false},
		{"bound summary denied", bound, "host:summary", false},
		{"wide scope still bound", wideScope, "host:box2:tick", false},
		{"wide scope own instance", wideScope, "host:box1:tick", true},
		{"wide scope summary denied", wideScope, "host:summary", false},
		{"unbound joins nothing", unbound, "host:box1:tick", false},
		{"binding must match scope", wrongBinding, "host:box1:tick", false},
		{"binding must match scope (own instance, no scope)", wrongBinding, "host:box2:tick", false},
	})
}

func TestJoinRoom_Can_device_host(t *testing.T) {
	deps := &authztest.FakeDeps{}
	bound := boundKey(authz.KindDevice, "d1", "box1", "room.join:host:box1:*")
	runJoinCases(t, deps, []joinCase{
		{"bound tick", bound, "host:box1:tick", true},
		{"bound other instance", bound, "host:box2:tick", false},
		{"bound bare denied", bound, "host:box1", false},
		{"bound summary denied", bound, "host:summary", false},
	})
}

// TestJoinRoom_Can_public: the public room type admits every kind but
// discord, credential or not; the anonymous Nobody() included.
func TestJoinRoom_Can_public(t *testing.T) {
	deps := &authztest.FakeDeps{}
	runJoinCases(t, deps, []joinCase{
		{"nobody", authz.Nobody(), "public:lobby", true},
		{"pb_user", pbUser("u1"), "public:lobby", true},
		{"machine", machine("k1"), "public:lobby", true},
		{"spectator", boundKey(authz.KindSpectator, "s1", "box1"), "public", true},
		{"device", boundKey(authz.KindDevice, "d1", "box1"), "public:lobby", true},
		{"discord", authz.Principal{Kind: authz.KindDiscord, ID: "snow"}, "public:lobby", false},
	})
}
