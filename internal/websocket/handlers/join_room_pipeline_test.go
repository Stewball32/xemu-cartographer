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

// TestJoinRoom_OverlayTokenPipeline proves a valid overlay token joins its bound
// host room through the full handler, while a missing/invalid/wrong-scope token
// — and the admin-only summary feed — is still rejected. Before the fix every
// row with a non-empty overlay token failed: CheckGuards' RequireAuth rejected
// the nil user before authorizeHostRoom ran.
func TestJoinRoom_OverlayTokenPipeline(t *testing.T) {
	tests := []struct {
		name        string
		joinRoom    string
		overlayRoom string // "" models a missing/invalid token (handshake bound none)
		wantJoined  bool
	}{
		{"valid token joins bound instance", "host:pod-a", "host:pod-a", true},
		{"valid token joins bound instance class sub-room", "host:pod-a:game", "host:pod-a", true},
		{"valid token joins another class of bound instance", "host:pod-a:xbox", "host:pod-a", true},
		{"wrong-scope token rejected", "host:pod-b", "host:pod-a", false},
		{"overlay token barred from summary feed", "host:summary", "host:pod-a", false},
		{"overlay token barred from legacy all feed", "host:all", "host:pod-a", false},
		{"missing/invalid token (nil user) rejected", "host:pod-a", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Services left nil: the admitted-join replay path early-returns on a
			// nil Scraper, and the rejection paths never reach it.
			out := runJoinRoom(&Event{Room: tc.joinRoom, OverlayRoom: tc.overlayRoom})
			if tc.wantJoined {
				if out.joined != tc.joinRoom {
					t.Fatalf("join %q (overlay %q): joined=%q errCode=%q errMsg=%q, want joined",
						tc.joinRoom, tc.overlayRoom, out.joined, out.errCode, out.errMsg)
				}
				return
			}
			if out.joined != "" {
				t.Fatalf("join %q (overlay %q): joined=%q, want rejection", tc.joinRoom, tc.overlayRoom, out.joined)
			}
			if out.errCode == "" {
				t.Fatalf("join %q (overlay %q): no error sent, want rejection", tc.joinRoom, tc.overlayRoom)
			}
		})
	}
}

// TestJoinRoom_OverlayBypassDoesNotLeakToNonHostRooms confirms the fix is scoped
// to host:* rooms: an overlay-token connection (nil user) targeting a non-host
// room still runs the room type's guard list, so it can't ride the overlay
// bypass into, e.g., the admin room.
func TestJoinRoom_OverlayBypassDoesNotLeakToNonHostRooms(t *testing.T) {
	out := runJoinRoom(&Event{Room: "admin:dashboard", OverlayRoom: "host:pod-a"})
	if out.joined != "" {
		t.Fatalf("overlay token joined %q, want rejection (non-host room runs its guards)", out.joined)
	}
	if out.errCode == "" {
		t.Fatal("overlay token joining admin:dashboard: no error sent, want rejection")
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
