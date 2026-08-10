package handlers

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	"github.com/Stewball32/xemu-cartographer/internal/guards"
)

// joinOutcome captures what handleJoinRoom did with an Event: the room it joined
// (via the JoinRoom closure) or the error it sent (via SendError).
type joinOutcome struct {
	joined  string
	errCode string
	errMsg  string
}

// runJoinRoom drives the whole join handler — Resolve → CheckGuards →
// authorizeHostRoom → JoinRoom — and reports the outcome. Exercising the full
// pipeline (not authorizeHostRoom in isolation) is what catches the M10 ordering
// bug: the host room's RequireAuth guard runs first in handleJoinRoom and, for
// an overlay-token connection (e.User == nil), used to reject the join before
// the overlay-scope bypass could admit it.
func runJoinRoom(e *Event) joinOutcome {
	var out joinOutcome
	e.JoinRoom = func(room string) { out.joined = room }
	e.SendError = func(code, message string) { out.errCode, out.errMsg = code, message }
	e.SendRaw = func([]byte) {}
	handleJoinRoom(e)
	return out
}

// TestJoinRoom_ConsolePipeline proves the tokenless console door's shape
// through the FULL handler: a console connection (nil user) targeting a host
// room skips the generic RequireAuth guard (authorizeHostRoom is then the sole
// authority — with no Services it fails closed), while the summary feeds and a
// bare anonymous connection are rejected. The happy admit path (console present
// in the live roster) is covered at the authorizeHostRoom/ContainerHasGamertag
// unit level (join_room_console_test.go) — it needs a live Membership view.
func TestJoinRoom_ConsolePipeline(t *testing.T) {
	tests := []struct {
		name     string
		joinRoom string
		console  string // "" models a plain anonymous connection
	}{
		{"console conn to instance room fails closed without services", "host:pod-a", "BlueBox"},
		{"console conn barred from summary feed", "host:summary", "BlueBox"},
		{"console conn barred from legacy all feed", "host:all", "BlueBox"},
		{"anonymous (no console) rejected by RequireAuth", "host:pod-a", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := runJoinRoom(&Event{Room: tc.joinRoom, ConsoleName: tc.console})
			if out.joined != "" {
				t.Fatalf("join %q (console %q): joined=%q, want rejection", tc.joinRoom, tc.console, out.joined)
			}
			if out.errCode == "" {
				t.Fatalf("join %q (console %q): no error sent, want rejection", tc.joinRoom, tc.console)
			}
		})
	}
}

// TestJoinRoom_ConsoleBypassDoesNotLeakToNonHostRooms confirms the guard bypass
// is scoped to host:* rooms: a console connection (nil user) targeting a
// non-host room still runs the room type's guard list, so it can't ride the
// console door into, e.g., the admin room.
func TestJoinRoom_ConsoleBypassDoesNotLeakToNonHostRooms(t *testing.T) {
	out := runJoinRoom(&Event{Room: "admin:dashboard", ConsoleName: "BlueBox"})
	if out.joined != "" {
		t.Fatalf("console conn joined %q, want rejection (non-host room runs its guards)", out.joined)
	}
	if out.errCode == "" {
		t.Fatal("console conn joining admin:dashboard: no error sent, want rejection")
	}
}

// TestJoinRoom_AdminUserJoinsHostRoom confirms the non-overlay path is unchanged:
// a superuser JWT (no overlay token) still flows through CheckGuards +
// authorizeHostRoom and joins a per-instance host room.
func TestJoinRoom_AdminUserJoinsHostRoom(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)

	col, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		t.Fatalf("superusers collection: %v", err)
	}
	su := core.NewRecord(col) // IsSuperuser()==true is collection-derived; no save needed.

	out := runJoinRoom(&Event{
		Room:     "host:pod-a",
		Services: &guards.Services{App: app},
		User:     su,
	})
	if out.joined != "host:pod-a" {
		t.Fatalf("superuser join host:pod-a: joined=%q errCode=%q errMsg=%q, want joined",
			out.joined, out.errCode, out.errMsg)
	}
}
