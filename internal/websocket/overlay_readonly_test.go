package websocket

import "testing"

// overlayAllowedType is the read-only whitelist for overlay-token connections:
// only room subscription, never control / request / probe types.
func TestOverlayAllowedType(t *testing.T) {
	allowed := []string{"join_room", "leave_room"}
	for _, ty := range allowed {
		if !overlayAllowedType(ty) {
			t.Errorf("overlayAllowedType(%q) = false, want true", ty)
		}
	}
	denied := []string{"request_state", "request_events", "request_probe", "", "anything", "broadcast"}
	for _, ty := range denied {
		if overlayAllowedType(ty) {
			t.Errorf("overlayAllowedType(%q) = true, want false (overlay tokens are read-only)", ty)
		}
	}
}
