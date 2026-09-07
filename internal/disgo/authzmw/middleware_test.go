package authzmw

import (
	"context"
	"fmt"
	"testing"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/handler"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/authz/authztest"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb"
)

const testGuild = "123456789"

// guildSlash builds a /config slash interaction from guild member 456 whose
// effective permissions are perms (the decimal bitfield Discord sends).
func guildSlash(t *testing.T, perms int64) discord.Interaction {
	t.Helper()
	raw := fmt.Sprintf(`{"type":2,"id":"1","application_id":"2","token":"t","version":1,
		"guild_id":%q,"member":{"user":{"id":"456","username":"stew"},"permissions":%q},
		"data":{"id":"10","name":"config","type":1}}`, testGuild, fmt.Sprint(perms))
	return unmarshalInteraction(t, raw)
}

// dmSlash builds a /box slash interaction sent from a DM (user, no member).
func dmSlash(t *testing.T) discord.Interaction {
	t.Helper()
	return unmarshalInteraction(t, `{"type":2,"id":"1","application_id":"2","token":"t","version":1,
		"user":{"id":"456","username":"stew"},"data":{"id":"11","name":"box","type":1}}`)
}

func unmarshalInteraction(t *testing.T, raw string) discord.Interaction {
	t.Helper()
	i, err := discord.UnmarshalInteraction([]byte(raw))
	if err != nil {
		t.Fatalf("unmarshal interaction: %v", err)
	}
	return i
}

// runMiddleware pushes i through Middleware and returns the context the
// downstream handler observed.
func runMiddleware(t *testing.T, deps func() *pb.PBDeps, i discord.Interaction) context.Context {
	t.Helper()
	var seen context.Context
	next := func(e *handler.InteractionEvent) error {
		seen = e.Ctx
		return nil
	}
	e := &handler.InteractionEvent{
		InteractionCreate: &events.InteractionCreate{Interaction: i},
		Vars:              map[string]string{},
		Ctx:               context.Background(),
	}
	if err := Middleware(deps)(next)(e); err != nil {
		t.Fatalf("middleware: %v", err)
	}
	if seen == nil {
		t.Fatal("middleware did not call next")
	}
	return seen
}

func TestManageGuildBit(t *testing.T) {
	deps := &authztest.FakeDeps{}
	guild := authz.Guild(testGuild)

	cases := []struct {
		name  string
		perms int64
		want  bool
	}{
		{"manage_guild", int64(discord.PermissionManageGuild), true},
		{"administrator", int64(discord.PermissionAdministrator), true},
		{"manage_guild_among_others", int64(discord.PermissionManageGuild | discord.PermissionSendMessages), true},
		{"send_messages_only", int64(discord.PermissionSendMessages), false},
		{"manage_channels_only", int64(discord.PermissionManageChannels), false},
		{"none", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := PrincipalFor(guildSlash(t, tc.perms))
			if p.Kind != authz.KindDiscord || p.ID != "456" || p.UserID != "" {
				t.Fatalf("principal = %+v", p)
			}
			if p.Extra[ExtraGuildID] != testGuild || p.Extra[ExtraMemberPermissions] != fmt.Sprint(tc.perms) {
				t.Fatalf("extra = %v", p.Extra)
			}
			for _, a := range []authz.Action{authz.ActionDiscordConfig, authz.ActionDiscordBindChannel} {
				if got := authz.Can(deps, p, a, guild); got != tc.want {
					t.Fatalf("%s: Can = %v, want %v (perms %d)", a, got, tc.want, tc.perms)
				}
			}
		})
	}

	// A DM has no member, hence no permissions: every Manage Server gate denies.
	dm := PrincipalFor(dmSlash(t))
	if dm.ID != "456" || dm.Extra[ExtraMemberPermissions] != "" {
		t.Fatalf("dm principal = %+v", dm)
	}
	if authz.Can(deps, dm, authz.ActionDiscordConfig, guild) {
		t.Fatal("DM caller passed discord.config")
	}

	// The middleware stores the principal in the event context and the
	// handlers read it back; without the middleware From is Nobody.
	ctx := runMiddleware(t, nil, guildSlash(t, int64(discord.PermissionManageGuild)))
	if got := From(ctx); got.Kind != authz.KindDiscord || got.ID != "456" {
		t.Fatalf("From(ctx) = %+v", got)
	}
	if got := From(context.Background()); got.Kind != authz.KindAnonymous || got.ID != "" {
		t.Fatalf("From(bare ctx) = %+v, want Nobody", got)
	}
	if got := From(nil); got.Kind != authz.KindAnonymous { //nolint:staticcheck // nil ctx is the fail-closed path under test
		t.Fatalf("From(nil) = %+v, want Nobody", got)
	}

	// Can(ctx) fails closed without deps (the middleware ran, the adapter
	// has not been installed yet) even for a member with Manage Server.
	if Can(ctx, authz.ActionDiscordConfig, guild) {
		t.Fatal("Can passed with nil deps")
	}
	if Deps(ctx) != nil {
		t.Fatal("Deps(ctx) should be nil when the getter is nil")
	}
	var called bool
	ctx = runMiddleware(t, func() *pb.PBDeps { called = true; return nil }, guildSlash(t, int64(discord.PermissionManageGuild)))
	if !called {
		t.Fatal("deps getter not resolved per interaction")
	}
	if Can(ctx, authz.ActionDiscordConfig, guild) {
		t.Fatal("Can passed with a nil *pb.PBDeps")
	}
}

func TestBoxDeniedUnlinked(t *testing.T) {
	deps := &authztest.FakeDeps{}

	// No _externalAuths link yet (A.9): the discord principal never carries
	// a UserID, so discord.box denies in a guild and in a DM alike.
	for name, i := range map[string]discord.Interaction{
		"guild": guildSlash(t, int64(discord.PermissionAdministrator)),
		"dm":    dmSlash(t),
	} {
		p := PrincipalFor(i)
		if p.UserID != "" {
			t.Fatalf("%s: UserID = %q, want empty", name, p.UserID)
		}
		if authz.Can(deps, p, authz.ActionDiscordBox, authz.Global()) {
			t.Fatalf("%s: unlinked caller passed discord.box", name)
		}
		if authz.Can(deps, p, authz.ActionDiscordBox, authz.Container("xc-play-u1")) {
			t.Fatalf("%s: unlinked caller passed discord.box on a container", name)
		}
	}

	// The gate is the link itself: the same principal with a linked user
	// passes (what A.9 will hand the middleware later).
	linked := PrincipalFor(dmSlash(t))
	linked.UserID = "u1"
	if !authz.Can(deps, linked, authz.ActionDiscordBox, authz.Global()) {
		t.Fatal("linked caller denied discord.box")
	}

	// Stats stay open to every guild caller, linked or not (D-4).
	if !authz.Can(deps, PrincipalFor(guildSlash(t, 0)), authz.ActionDiscordStatsRead, authz.Guild(testGuild)) {
		t.Fatal("unlinked caller denied discord.stats.read")
	}
}
