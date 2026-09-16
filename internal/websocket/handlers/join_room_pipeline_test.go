package handlers

import (
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/authz/authztest"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb"
	"github.com/Stewball32/xemu-cartographer/internal/guards"
	scraperiface "github.com/Stewball32/xemu-cartographer/internal/guards/interfaces/scraper"
)

// joinOutcome captures what handleJoinRoom did with an Event: the room it joined
// (via the JoinRoom closure) or the error it sent (via SendError).
type joinOutcome struct {
	joined  string
	errCode string
	errMsg  string
}

// runJoinRoom drives the whole join handler — Resolve → ParseRoom →
// authz.Can(room.join) → JoinRoom → replay — and reports the outcome.
// Exercising the full pipeline (not the decision in isolation) is what
// catches ordering bugs: the pre-authz handler once ran a room type's
// RequireAuth guard ahead of the console-door bypass and rejected the
// join before the bypass could admit it.
func runJoinRoom(e *Event) joinOutcome {
	var out joinOutcome
	e.JoinRoom = func(room string) { out.joined = room }
	e.SendError = func(code, message string) { out.errCode, out.errMsg = code, message }
	if e.SendRaw == nil {
		e.SendRaw = func([]byte) {}
	}
	handleJoinRoom(e)
	return out
}

// TestJoinRoom_Pipeline_ErrorCodes pins the error a client sees for each
// way a join can fail before the decision: an unregistered type is
// "not_found" (the registry has the final say on non-host types), a
// malformed host room is "bad_room", and nothing is ever joined. A
// superuser is used so a rejection cannot be an authz denial in disguise.
func TestJoinRoom_Pipeline_ErrorCodes(t *testing.T) {
	deps := &authztest.FakeDeps{}
	su := authz.Superuser("su")
	tests := []struct {
		room     string
		wantCode string
	}{
		{"bogus:x", "not_found"},
		{"bogus", "not_found"},
		{"host", "bad_room"},
		{"host:", "bad_room"},
		{"host:all:tick", "bad_room"},
		{"host:summary:game", "bad_room"},
		{"host:pod-a:bogus", "bad_room"},
		{"host:pod-a:tick:extra", "bad_room"},
		{"host:pod a", "bad_room"},
	}
	for _, tc := range tests {
		t.Run(tc.room, func(t *testing.T) {
			out := runJoinRoom(&Event{Room: tc.room, Authz: deps, Principal: su})
			if out.joined != "" {
				t.Fatalf("join %q: joined=%q, want rejection", tc.room, out.joined)
			}
			if out.errCode != tc.wantCode {
				t.Fatalf("join %q: errCode=%q (%q), want %q", tc.room, out.errCode, out.errMsg, tc.wantCode)
			}
		})
	}

	// An empty room is a no-op: nothing joined, nothing sent.
	out := runJoinRoom(&Event{Room: "", Authz: deps, Principal: su})
	if out.joined != "" || out.errCode != "" {
		t.Fatalf("empty room: joined=%q errCode=%q, want silent no-op", out.joined, out.errCode)
	}
}

// TestJoinRoom_NilDepsFailsClosed: with no authz adapter — nil, or a
// typed-nil *pb.PBDeps the way pb.Default() reads before boot — every
// join is forbidden, superuser included. The Hub never hands a handler
// a nil Authz once the adapter is installed; this is the guarantee for
// the window before it is.
func TestJoinRoom_NilDepsFailsClosed(t *testing.T) {
	var typedNil *pb.PBDeps
	for name, deps := range map[string]authz.Deps{"nil": nil, "typed-nil": typedNil} {
		t.Run(name, func(t *testing.T) {
			for _, room := range []string{"host:pod-a", "host:summary", "public:lobby", "admin:dashboard"} {
				out := runJoinRoom(&Event{Room: room, Authz: deps, Principal: authz.Superuser("su")})
				if out.joined != "" || out.errCode != "forbidden" {
					t.Errorf("superuser join %q with %s deps: joined=%q errCode=%q, want forbidden",
						room, name, out.joined, out.errCode)
				}
			}
		})
	}
}

// stubReplayScraper records which replay a join asked the scraper for.
type stubReplayScraper struct {
	scraperiface.Service
	instance []string
	class    []string
	hostAll  int
}

func (s *stubReplayScraper) JoinReplayForInstance(name string) [][]byte {
	s.instance = append(s.instance, name)
	return [][]byte{[]byte("instance:" + name)}
}

func (s *stubReplayScraper) JoinReplayForInstanceClass(name, class string) [][]byte {
	s.class = append(s.class, name+":"+class)
	return [][]byte{[]byte("class:" + name + ":" + class)}
}

func (s *stubReplayScraper) JoinReplayForHostAll() [][]byte {
	s.hostAll++
	return [][]byte{[]byte("hostall")}
}

// TestJoinRoom_Pipeline_Replay: an admitted join replays the right
// snapshot for the room's shape — the runner's full cache for a bare
// instance room, the per-class slice for a class room, the hosts list for
// either aggregate — and a refused join replays nothing.
func TestJoinRoom_Pipeline_Replay(t *testing.T) {
	deps := &authztest.FakeDeps{Rostered: map[string]bool{authztest.Key("u1", "pod-a"): true}}
	member := pbUser("u1")
	admin := pbUser("a1", "admin.*", "room.join:*")

	run := func(p authz.Principal, room string) (*stubReplayScraper, []string, joinOutcome) {
		stub := &stubReplayScraper{}
		var sent []string
		out := runJoinRoom(&Event{
			Room:      room,
			Authz:     deps,
			Principal: p,
			Services:  &guards.Services{Scraper: stub},
			SendRaw:   func(b []byte) { sent = append(sent, string(b)) },
		})
		return stub, sent, out
	}

	stub, sent, out := run(member, "host:pod-a")
	if out.joined != "host:pod-a" || !equalStrings(stub.instance, []string{"pod-a"}) || len(stub.class) != 0 || stub.hostAll != 0 {
		t.Fatalf("bare room: joined=%q instance=%v class=%v hostAll=%d", out.joined, stub.instance, stub.class, stub.hostAll)
	}
	if !equalStrings(sent, []string{"instance:pod-a"}) {
		t.Fatalf("bare room replay sent %v", sent)
	}

	stub, sent, out = run(member, "host:pod-a:tick")
	if out.joined != "host:pod-a:tick" || len(stub.instance) != 0 || !equalStrings(stub.class, []string{"pod-a:tick"}) || stub.hostAll != 0 {
		t.Fatalf("class room: joined=%q instance=%v class=%v hostAll=%d", out.joined, stub.instance, stub.class, stub.hostAll)
	}
	if !equalStrings(sent, []string{"class:pod-a:tick"}) {
		t.Fatalf("class room replay sent %v", sent)
	}

	for _, room := range []string{"host:summary", "host:all"} {
		stub, sent, out = run(admin, room)
		if out.joined != room || len(stub.instance) != 0 || len(stub.class) != 0 || stub.hostAll != 1 {
			t.Fatalf("%s: joined=%q instance=%v class=%v hostAll=%d", room, out.joined, stub.instance, stub.class, stub.hostAll)
		}
		if !equalStrings(sent, []string{"hostall"}) {
			t.Fatalf("%s replay sent %v", room, sent)
		}
	}

	stub, sent, out = run(member, "host:pod-b")
	if out.joined != "" || out.errCode != "forbidden" || len(stub.instance)+len(stub.class)+stub.hostAll != 0 || len(sent) != 0 {
		t.Fatalf("refused join replayed: joined=%q errCode=%q instance=%v class=%v hostAll=%d sent=%v",
			out.joined, out.errCode, stub.instance, stub.class, stub.hostAll, sent)
	}
}

// TestJoinRoom_AdminUserJoinsHostRoom confirms the privileged path is
// unchanged: a superuser needs no roster to join a per-instance host room,
// and so does a user holding the admin role's scopes.
func TestJoinRoom_AdminUserJoinsHostRoom(t *testing.T) {
	deps := &authztest.FakeDeps{}
	for _, p := range []authz.Principal{authz.Superuser("su"), pbUser("a1", "admin.*", "room.join:*", "scraper.*")} {
		out := runJoinRoom(&Event{Room: "host:pod-a", Authz: deps, Principal: p})
		if out.joined != "host:pod-a" {
			t.Fatalf("%s join host:pod-a: joined=%q errCode=%q errMsg=%q, want joined",
				p.Kind, out.joined, out.errCode, out.errMsg)
		}
	}
}
