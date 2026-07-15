package commands

import (
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/discordcfg"
)

// TestSetupChannelSpec verifies the fixed channel set + that each Discord channel
// name maps to a distinct, valid post-hook (the pure part of /setup; the Discord
// REST provisioning + binding writes are thin plumbing verified live).
func TestSetupChannelSpec(t *testing.T) {
	spec := setupChannelSpec()
	want := map[string]string{
		"container-status": discordcfg.HookContainerStatus,
		"kiosk-links":      discordcfg.HookKioskLinks,
		"announcements":    discordcfg.HookAnnouncements,
		"bot-log":          discordcfg.HookBotLog,
	}
	if len(spec) != len(want) {
		t.Fatalf("spec has %d channels, want %d", len(spec), len(want))
	}
	postHooks := map[string]bool{}
	for _, h := range discordcfg.PostHooks {
		postHooks[h] = true
	}
	seen := map[string]bool{}
	for _, s := range spec {
		if want[s.name] != s.hook {
			t.Errorf("channel %q → hook %q, want %q", s.name, s.hook, want[s.name])
		}
		if !postHooks[s.hook] {
			t.Errorf("hook %q for %q is not in PostHooks", s.hook, s.name)
		}
		if seen[s.hook] {
			t.Errorf("hook %q bound more than once", s.hook)
		}
		seen[s.hook] = true
	}
}
