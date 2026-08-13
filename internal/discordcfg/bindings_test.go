package discordcfg_test

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	"github.com/Stewball32/xemu-cartographer/internal/discordcfg"
)

// newBindingsApp spins a bare test PB with just the canonical discord_routes
// collection (mirrors internal/pocketbase/schema/discord_routes.go — we don't
// import schema, which would trigger every collection's init()). `hook` is
// free-text per the Discord bot spec.
func newBindingsApp(t *testing.T) core.App {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(func() { app.Cleanup() })

	col := core.NewBaseCollection(discordcfg.RoutesCollection)
	col.Fields.Add(
		&core.TextField{Name: "guild_id", Required: true, Max: 32},
		&core.TextField{Name: "hook", Required: true, Max: 64},
		&core.TextField{Name: "channel_id", Required: true, Max: 32},
	)
	col.AddIndex("idx_test_guild_hook", true, "guild_id, hook", "")
	if err := app.Save(col); err != nil {
		t.Fatalf("save collection: %v", err)
	}
	return app
}

func TestBindingsRoundTrip(t *testing.T) {
	app := newBindingsApp(t)
	const g = "guild-1"

	if _, ok := discordcfg.GetBinding(app, g, discordcfg.HookBotLog); ok {
		t.Fatal("expected no binding before setup")
	}

	if err := discordcfg.SetBinding(app, g, discordcfg.HookContainerStatus, "chan-status"); err != nil {
		t.Fatalf("SetBinding: %v", err)
	}
	if err := discordcfg.SetBinding(app, g, discordcfg.HookBotLog, "chan-log"); err != nil {
		t.Fatalf("SetBinding: %v", err)
	}

	if ch, ok := discordcfg.GetBinding(app, g, discordcfg.HookContainerStatus); !ok || ch != "chan-status" {
		t.Errorf("GetBinding = %q,%v; want chan-status,true", ch, ok)
	}

	// Upsert: same (guild, hook) updates in place (no duplicate row).
	if err := discordcfg.SetBinding(app, g, discordcfg.HookContainerStatus, "chan-status-2"); err != nil {
		t.Fatalf("SetBinding upsert: %v", err)
	}
	if ch, _ := discordcfg.GetBinding(app, g, discordcfg.HookContainerStatus); ch != "chan-status-2" {
		t.Errorf("upsert didn't update: %q", ch)
	}

	m, err := discordcfg.GetBindings(app, g)
	if err != nil {
		t.Fatalf("GetBindings: %v", err)
	}
	if len(m) != 2 || m[discordcfg.HookContainerStatus] != "chan-status-2" || m[discordcfg.HookBotLog] != "chan-log" {
		t.Errorf("GetBindings = %v", m)
	}

	// Isolation: a different guild sees nothing.
	if m2, _ := discordcfg.GetBindings(app, "guild-2"); len(m2) != 0 {
		t.Errorf("guild-2 should have no bindings, got %v", m2)
	}
}
