package commands

import (
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/discordcfg"
)

// TestSetupChannelSpec verifies the fixed channel set + that each spec assigns
// its created ID to the right binding field (the pure part of /setup; the
// Discord REST provisioning is thin plumbing verified live).
func TestSetupChannelSpec(t *testing.T) {
	spec := setupChannelSpec()
	wantNames := []string{"container-status", "kiosk-links", "announcements", "bot-log"}
	if len(spec) != len(wantNames) {
		t.Fatalf("spec has %d channels, want %d", len(spec), len(wantNames))
	}

	var ch discordcfg.Channels
	for i, s := range spec {
		if s.name != wantNames[i] {
			t.Errorf("spec[%d].name = %q, want %q", i, s.name, wantNames[i])
		}
		s.assign(&ch, "id-"+s.name)
	}
	// Each assign wrote its distinct field.
	if ch.ContainerStatus != "id-container-status" ||
		ch.KioskLinks != "id-kiosk-links" ||
		ch.Announcements != "id-announcements" ||
		ch.BotLog != "id-bot-log" {
		t.Errorf("assign fns didn't map to distinct fields: %+v", ch)
	}
}
