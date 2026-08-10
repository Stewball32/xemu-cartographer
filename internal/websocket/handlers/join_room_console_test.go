package handlers

import "testing"

// authorizeHostRoom's console branch: the pure (no-DB) cases. The admit path
// (console present in the instance's live roster) goes through
// scraperiface.ContainerHasGamertag, which is unit-tested in the scraper iface
// package — here we cover the deny/pass-through decisions that don't need a live
// Membership view, mirroring the overlay-scoping test.
func TestAuthorizeHostRoom_ConsoleScoping(t *testing.T) {
	tests := []struct {
		name        string
		console     string
		joinRoom    string
		wantDenied  bool
		description string
	}{
		{"summary feed denied", "BlueBox", "host:summary", true, "console overlays can't watch the admin summary"},
		{"legacy all feed denied", "BlueBox", "host:all", true, "console overlays can't watch the aggregate"},
		{"instance room, no services → denied", "BlueBox", "host:pod-a", true, "fails closed without a Membership view"},
		{"non-host room pass-through", "BlueBox", "admin:dashboard", false, "not a host room — left to the room type's guards"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// No Services → the instance branch fails closed; summary/legacy are
			// rejected before any lookup; non-host returns nil (pass-through).
			e := &Event{Room: tc.joinRoom, ConsoleName: tc.console}
			err := authorizeHostRoom(e)
			if tc.wantDenied && err == nil {
				t.Errorf("join %q with console %q: got allowed, want denied (%s)", tc.joinRoom, tc.console, tc.description)
			}
			if !tc.wantDenied && err != nil {
				t.Errorf("join %q with console %q: got error %v, want allowed (%s)", tc.joinRoom, tc.console, err, tc.description)
			}
		})
	}
}
