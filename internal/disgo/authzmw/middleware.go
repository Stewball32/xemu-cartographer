// Package authzmw derives the authz principal for every Discord interaction
// (design §7.2 D-1) so the command handlers can ask authz.Can against the
// caller's server-side member permissions instead of trusting the
// client-side DefaultMemberPermissions default alone.
//
// bot.go installs Middleware on the handler mux; it runs before every
// route, builds the KindDiscord principal from the interaction's resolved
// member and stores it (plus the deps to decide against) in the event
// context. Handlers read them back through From / Deps or ask Can directly.
package authzmw

import (
	"context"
	"strconv"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb"
)

// Extra keys the discord principal carries (authz.Principal.Extra).
const (
	ExtraGuildID           = "guild_id"
	ExtraMemberPermissions = "member_permissions"
)

type principalKey struct{}

type depsKey struct{}

// Middleware returns the handler.Middleware bot.go attaches with mux.Use.
// deps is called per interaction (bot.go passes pb.Default) so the mux can
// be built before boot installs the adapter; a nil result makes every
// decision deny ("no_deps"), which is the fail-closed default.
func Middleware(deps func() *pb.PBDeps) handler.Middleware {
	return func(next handler.Handler) handler.Handler {
		return func(e *handler.InteractionEvent) error {
			ctx := e.Ctx
			if ctx == nil {
				ctx = context.Background()
			}
			ctx = context.WithValue(ctx, principalKey{}, PrincipalFor(e.Interaction))
			var d *pb.PBDeps
			if deps != nil {
				d = deps()
			}
			e.Ctx = context.WithValue(ctx, depsKey{}, d)
			return next(e)
		}
	}
}

// PrincipalFor builds the KindDiscord principal for an interaction: ID is
// the caller's user snowflake; Extra carries the guild and the caller's
// effective permissions in that guild (decimal, as Discord serialises them)
// when the interaction came from a guild member. A DM interaction has no
// member and therefore no permissions, so every Manage Server gate denies.
// UserID stays empty until the _externalAuths link lands (A.9), which is
// what keeps discord.box denied for now.
func PrincipalFor(i discord.Interaction) authz.Principal {
	p := authz.Principal{Kind: authz.KindDiscord, Extra: map[string]string{}}
	if i == nil {
		return p
	}
	if gid := i.GuildID(); gid != nil {
		p.Extra[ExtraGuildID] = gid.String()
	}
	if m := i.Member(); m != nil {
		p.ID = m.User.ID.String()
		p.Extra[ExtraMemberPermissions] = strconv.FormatUint(uint64(m.Permissions), 10)
		return p
	}
	p.ID = i.User().ID.String()
	return p
}

// From returns the principal Middleware stored in ctx, or authz.Nobody()
// when the middleware did not run (fail-closed: Nobody passes no discord
// rule).
func From(ctx context.Context) authz.Principal {
	if ctx == nil {
		return authz.Nobody()
	}
	if p, ok := ctx.Value(principalKey{}).(authz.Principal); ok {
		return p
	}
	return authz.Nobody()
}

// Deps returns the deps Middleware resolved for this interaction; nil when
// the middleware did not run or the adapter is not installed yet.
func Deps(ctx context.Context) *pb.PBDeps {
	if ctx == nil {
		return nil
	}
	d, _ := ctx.Value(depsKey{}).(*pb.PBDeps)
	return d
}

// Can is authz.Can(Deps(ctx), From(ctx), a, r): the one-liner the command
// handlers gate on.
func Can(ctx context.Context, a authz.Action, r authz.Resource) bool {
	return authz.Can(Deps(ctx), From(ctx), a, r)
}
