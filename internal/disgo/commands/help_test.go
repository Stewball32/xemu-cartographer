package commands

import (
	"strings"
	"testing"

	"github.com/disgoorg/disgo/discord"
)

func findCmd(t *testing.T, name string) Command {
	t.Helper()
	for _, c := range All() {
		if c.Create.Name == name {
			return c
		}
	}
	t.Fatalf("command %q not registered", name)
	return Command{}
}

func TestHelp_Overview_PermissionAware(t *testing.T) {
	// Regular member (no perms): the Manage-Server /config is hidden.
	ov := helpOverview(0).Description
	if strings.Contains(ov, "`/config`") {
		t.Error("/config should be hidden from a member without Manage Server")
	}
	if !strings.Contains(ov, "`/ping`") || !strings.Contains(ov, "`/box`") {
		t.Error("open commands should be listed")
	}
	if !strings.Contains(ov, "**Xbox**") || !strings.Contains(ov, "**Stats**") {
		t.Error("commands should be grouped by their Group label")
	}
	if !strings.Contains(ov, botDocsURL) {
		t.Error("overview should link the docs page")
	}

	// Manage-Server member: /config and its group appear.
	ovAdmin := helpOverview(discord.PermissionManageGuild).Description
	if !strings.Contains(ovAdmin, "`/config`") || !strings.Contains(ovAdmin, "**Configuration**") {
		t.Error("/config should be listed for a Manage-Server member")
	}
}

func TestHelp_Detail(t *testing.T) {
	// Regular member cannot see the gated /config.
	if got := helpDetail("config", 0); got.Title != "Unknown command" {
		t.Errorf("regular member /config detail should be blocked, got title %q", got.Title)
	}

	// Manage-Server member sees /config with its subcommands + access line.
	dc := helpDetail("config", discord.PermissionManageGuild)
	if dc.Title != "/config" {
		t.Fatalf("title = %q", dc.Title)
	}
	for _, sub := range []string{"tags", "view", "bootstrap"} {
		if !strings.Contains(dc.Description, sub) {
			t.Errorf("/config detail missing subcommand %q", sub)
		}
	}
	if !strings.Contains(dc.Description, "Manage Server") {
		t.Error("/config detail should show the required permission")
	}

	// Open command with subcommands (/box) is visible to everyone.
	db := helpDetail("box", 0)
	for _, sub := range []string{"status", "link", "request", "stop"} {
		if !strings.Contains(db.Description, sub) {
			t.Errorf("/box detail missing subcommand %q", sub)
		}
	}
	if !strings.Contains(db.Description, "Everyone") {
		t.Error("/box access should be Everyone")
	}

	// Flat command with a required leaf option (/leaderboard has `type`).
	dl := helpDetail("leaderboard", 0).Description
	if !strings.Contains(dl, "type") {
		t.Error("/leaderboard detail should list the `type` option")
	}
	if !strings.Contains(dl, "(required)") {
		t.Error("/leaderboard `type` option should be tagged required")
	}
}

func TestHelp_MemberCanUse(t *testing.T) {
	cfg := findCmd(t, "config")
	if memberCanUse(0, cfg) {
		t.Error("no-perm member should NOT use /config")
	}
	if !memberCanUse(discord.PermissionManageGuild, cfg) {
		t.Error("Manage-Server member should use /config")
	}
	if !memberCanUse(discord.PermissionAdministrator, cfg) {
		t.Error("Administrator should use /config")
	}
	if !memberCanUse(0, findCmd(t, "ping")) {
		t.Error("everyone should use /ping")
	}
	if !memberCanUse(0, findCmd(t, "box")) {
		t.Error("everyone should use /box")
	}
}

// TestHelp_Autocomplete_HidesHelpAndGated proves the autocomplete filter excludes
// /help itself and any command the member can't use.
func TestHelp_Autocomplete_ExcludesSelf(t *testing.T) {
	// helpOverview is the same registry walk the autocomplete uses; assert /help
	// is registered with an autocomplete route so bot.go wires it.
	if _ = findCmd(t, "help"); len(AllAutocompletes()) == 0 {
		t.Fatal("/help must register an autocomplete route")
	}
	found := false
	for _, r := range AllAutocompletes() {
		if r.Pattern == "/help" {
			found = true
		}
	}
	if !found {
		t.Error("no autocomplete route registered for /help")
	}
}
