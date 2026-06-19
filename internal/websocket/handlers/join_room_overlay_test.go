package handlers

import "testing"

// authorizeHostRoom's overlay branch is pure (no DB): an overlay-token
// connection is scoped to its bound instance and barred from the summary feed.
func TestAuthorizeHostRoom_OverlayScoping(t *testing.T) {
	tests := []struct {
		name        string
		overlayRoom string
		joinRoom    string
		wantAllowed bool
	}{
		{"bound instance room", "host:pod-a", "host:pod-a", true},
		{"bound instance class sub-room", "host:pod-a", "host:pod-a:game", true},
		{"bound instance other class", "host:pod-a", "host:pod-a:xbox", true},
		{"different instance denied", "host:pod-a", "host:pod-b", false},
		{"summary feed denied", "host:pod-a", "host:summary", false},
		{"legacy all feed denied", "host:pod-a", "host:all", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := &Event{Room: tc.joinRoom, OverlayRoom: tc.overlayRoom}
			err := authorizeHostRoom(e)
			if tc.wantAllowed && err != nil {
				t.Errorf("join %q with overlay %q: got error %v, want allowed", tc.joinRoom, tc.overlayRoom, err)
			}
			if !tc.wantAllowed && err == nil {
				t.Errorf("join %q with overlay %q: got allowed, want denied", tc.joinRoom, tc.overlayRoom)
			}
		})
	}
}
