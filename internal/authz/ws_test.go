package authz

import (
	"errors"
	"fmt"
	"testing"
)

// TestParseRoom pins the §3.5 room grammar.
func TestParseRoom(t *testing.T) {
	good := map[string]Room{
		"host:all":                {Type: "host", Instance: "all", Raw: "host:all"},
		"host:summary":            {Type: "host", Instance: "summary", Raw: "host:summary"},
		"host:box1":               {Type: "host", Instance: "box1", Raw: "host:box1"},
		"host:box1:tick":          {Type: "host", Instance: "box1", Class: "tick", Raw: "host:box1:tick"},
		"host:box1:game_filtered": {Type: "host", Instance: "box1", Class: "game_filtered", Raw: "host:box1:game_filtered"},
		"host:Box-1_a.b:xbox":     {Type: "host", Instance: "Box-1_a.b", Class: "xbox", Raw: "host:Box-1_a.b:xbox"},
		"admin":                   {Type: "admin", Raw: "admin"},
		"public":                  {Type: "public", Raw: "public"},
		"lobby":                   {Type: "lobby", Raw: "lobby"},
		"foo:bar":                 {Type: "foo", Raw: "foo:bar"},
		"admin:":                  {Type: "admin", Raw: "admin:"},
	}
	for in, want := range good {
		got, err := ParseRoom(in)
		if err != nil {
			t.Errorf("ParseRoom(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseRoom(%q) = %+v, want %+v", in, got, want)
		}
		if got.Selector() != in && got.IsHost() {
			t.Errorf("ParseRoom(%q).Selector() = %q, want round-trip", in, got.Selector())
		}
	}

	bad := []string{
		"",
		":",
		":box1",
		"host",
		"host:",
		"host::tick",
		"host:a:b:c",
		"host:box1:",
		"host:box1:nope",
		"host:box1:Tick",
		"host:box1:tick:extra",
		"host:all:tick",
		"host:summary:game_filtered",
		"host:all:",
		"host:box 1",
		"host:box\t1:tick",
		"host: box1",
		"host:box1 ",
		"host: box1",
	}
	for _, in := range bad {
		if got, err := ParseRoom(in); !errors.Is(err, ErrRoomSyntax) {
			t.Errorf("ParseRoom(%q) = %+v, %v; want ErrRoomSyntax", in, got, err)
		} else if got != (Room{}) {
			t.Errorf("ParseRoom(%q) returned a room alongside the error: %+v", in, got)
		}
	}

	// Aggregates.
	for _, in := range []string{"host:all", "host:summary"} {
		rm, _ := ParseRoom(in)
		if !rm.IsHost() || !rm.IsHostAggregate() {
			t.Errorf("%q should be a host aggregate: %+v", in, rm)
		}
	}
	if rm, _ := ParseRoom("host:box1"); rm.IsHostAggregate() || !rm.IsHost() {
		t.Errorf("host:box1 should be a plain host room: %+v", rm)
	}
	if rm, _ := ParseRoom("admin"); rm.IsHost() || rm.IsHostAggregate() {
		t.Errorf("admin should not be a host room: %+v", rm)
	}

	// Every registered scraper class parses and nothing else does.
	classes := ScraperClasses()
	if len(classes) != len(scraperClasses) || len(classes) == 0 {
		t.Fatalf("ScraperClasses() = %v", classes)
	}
	for i, c := range classes {
		if i > 0 && classes[i-1] >= c {
			t.Errorf("ScraperClasses() not sorted at %d: %v", i, classes)
		}
		if _, err := ParseRoom("host:box1:" + c); err != nil {
			t.Errorf("class %q should parse: %v", c, err)
		}
	}
	// Mutating the returned slice must not affect the registry.
	classes[0] = "mutated"
	if _, err := ParseRoom("host:box1:mutated"); err == nil {
		t.Error("ScraperClasses() must return a copy")
	}
}

// TestWSSendAllowed moves the overlay_readonly_test split into authz: bound
// read-only kinds may subscribe and request replays, anonymous may only
// subscribe, discord and unknown kinds may send nothing.
func TestWSSendAllowed(t *testing.T) {
	allowedAnon := []string{"join_room", "leave_room"}
	deniedAnon := []string{"request_state", "request_events", "request_probe", "", "anything", "broadcast"}
	for _, ty := range allowedAnon {
		if !WSSendAllowed(KindAnonymous, ty) {
			t.Errorf("WSSendAllowed(anonymous, %q) = false, want true", ty)
		}
	}
	for _, ty := range deniedAnon {
		if WSSendAllowed(KindAnonymous, ty) {
			t.Errorf("WSSendAllowed(anonymous, %q) = true, want false (anonymous may only subscribe)", ty)
		}
	}

	readOnly := []string{"join_room", "leave_room", "request_state", "request_events"}
	deniedReadOnly := []string{"request_probe", "", "anything", "broadcast", "JOIN_ROOM", " join_room"}
	for _, k := range []Kind{KindSpectator, KindDevice} {
		for _, ty := range readOnly {
			if !WSSendAllowed(k, ty) {
				t.Errorf("WSSendAllowed(%s, %q) = false, want true", k, ty)
			}
		}
		for _, ty := range deniedReadOnly {
			if WSSendAllowed(k, ty) {
				t.Errorf("WSSendAllowed(%s, %q) = true, want false (read-only kinds)", k, ty)
			}
		}
	}

	full := []Kind{KindPBUser, KindSuperuser, KindInternal, KindMachine}
	for _, k := range full {
		for _, ty := range append(append([]string{}, readOnly...), "request_probe", "anything", "broadcast") {
			if !WSSendAllowed(k, ty) {
				t.Errorf("WSSendAllowed(%s, %q) = false, want true", k, ty)
			}
		}
		if WSSendAllowed(k, "") {
			t.Errorf("WSSendAllowed(%s, \"\") = true, want false", k)
		}
	}

	for _, k := range []Kind{KindDiscord, Kind(""), Kind("martian")} {
		for _, ty := range append(append([]string{}, readOnly...), "request_probe", "", "anything") {
			if WSSendAllowed(k, ty) {
				t.Errorf("WSSendAllowed(%q, %q) = true, want false", k, ty)
			}
		}
	}
}

func TestJoinableInstances(t *testing.T) {
	all := []string{"box1", "box2", "box3"}
	fmtList := func(v []string) string { return fmt.Sprint(v) }

	// Full-visibility kinds get an owned copy of everything.
	for _, p := range []Principal{
		{Kind: KindPBUser, ID: "u1"},
		Superuser("s"),
		Internal("x"),
		{Kind: KindMachine, ID: "mk_1"},
	} {
		got := JoinableInstances(p, all)
		if fmtList(got) != fmtList(all) {
			t.Errorf("%s: got %v", p.Kind, got)
		}
		got[0] = "mutated"
		if all[0] != "box1" {
			t.Fatalf("%s: JoinableInstances must copy", p.Kind)
		}
	}

	// Bound kinds see only their binding, and only when it is live.
	for _, k := range []Kind{KindSpectator, KindDevice, KindAnonymous} {
		bound := Principal{Kind: k, Bound: map[string]string{"instance": "box2"}}
		if got := JoinableInstances(bound, all); fmtList(got) != "[box2]" {
			t.Errorf("%s bound box2: got %v", k, got)
		}
		gone := Principal{Kind: k, Bound: map[string]string{"instance": "box9"}}
		if got := JoinableInstances(gone, all); got == nil || len(got) != 0 {
			t.Errorf("%s bound to a dead instance: got %v", k, got)
		}
		unbound := Principal{Kind: k}
		if got := JoinableInstances(unbound, all); got == nil || len(got) != 0 {
			t.Errorf("%s unbound: got %v", k, got)
		}
		// A wildcard scope does not widen the binding.
		wide := Principal{Kind: k, Scopes: []string{"*"}, Bound: map[string]string{"instance": "box1"}}
		if got := JoinableInstances(wide, all); fmtList(got) != "[box1]" {
			t.Errorf("%s with * scope: got %v", k, got)
		}
		// The binding is matched under the scope segment fold (predBound
		// does the same) and reported by the live instance's own name.
		cased := Principal{Kind: k, Bound: map[string]string{"instance": "Box2"}}
		if got := JoinableInstances(cased, []string{"box1", "box2"}); fmtList(got) != "[box2]" {
			t.Errorf("%s bound Box2 against box2: got %v", k, got)
		}
		if got := JoinableInstances(cased, []string{"box1", "BOX2"}); fmtList(got) != "[BOX2]" {
			t.Errorf("%s bound Box2 against BOX2: got %v", k, got)
		}
		if got := JoinableInstances(cased, []string{"box1", "box22"}); got == nil || len(got) != 0 {
			t.Errorf("%s bound Box2 against box22: got %v", k, got)
		}
	}

	// Discord, unknown and zero principals see nothing; nil input never yields nil.
	for _, p := range []Principal{{Kind: KindDiscord, ID: "1"}, {Kind: "martian"}, {}} {
		if got := JoinableInstances(p, all); got == nil || len(got) != 0 {
			t.Errorf("%q: got %v", p.Kind, got)
		}
	}
	if got := JoinableInstances(Principal{Kind: KindPBUser}, nil); got == nil || len(got) != 0 {
		t.Errorf("pb_user with nil all: got %v", got)
	}
	if got := JoinableInstances(Nobody(), nil); got == nil || len(got) != 0 {
		t.Errorf("Nobody with nil all: got %v", got)
	}
}
