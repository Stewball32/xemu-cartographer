package commands

import (
	"strings"
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/authz/authztest"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb"
)

// TestBoxStatusEmbed covers the pure /box status embed builder for each
// resolution outcome (the live resolution itself is thin plumbing verified on a
// connected gateway).
func TestBoxStatusEmbed(t *testing.T) {
	cases := []struct {
		name      string
		container string
		res       resolveResult
		wantTitle string
		wantIn    string // substring expected in the description
	}{
		{"matched", "beta-play-abc", resolveMatched, "Your box", "beta-play-abc"},
		{"idle", "", resolveIdle, "Your box", "not in a live match"},
		{"not linked", "", resolveNotLinked, "Not linked", "isn't linked"},
		{"unavailable", "", resolveUnavailable, "Unavailable", "isn't running"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			emb := boxStatusEmbed(c.container, c.res)
			if emb.Title != c.wantTitle {
				t.Errorf("title = %q, want %q", emb.Title, c.wantTitle)
			}
			if !strings.Contains(emb.Description, c.wantIn) {
				t.Errorf("description %q missing %q", emb.Description, c.wantIn)
			}
		})
	}
}

// TestResolveCallerContainerDeniedUnlinked pins the D-3 gate: the authz
// `discord.box` decision runs before any account lookup, so an unlinked
// Discord caller (no UserID on the principal — every caller until the
// _externalAuths link lands, A.9), a request the mux middleware did not see
// (Nobody) or a missing adapter all resolve to "not linked" even though
// Services are not wired (which would otherwise answer "unavailable").
func TestResolveCallerContainerDeniedUnlinked(t *testing.T) {
	deps := &authztest.FakeDeps{}
	unlinked := authz.Principal{
		Kind:  authz.KindDiscord,
		ID:    "456",
		Extra: map[string]string{"guild_id": "123", "member_permissions": "8"},
	}
	linked := unlinked
	linked.UserID = "u1"

	cases := []struct {
		name string
		deps authz.Deps
		p    authz.Principal
		want resolveResult
	}{
		{"unlinked", deps, unlinked, resolveNotLinked},
		{"no middleware", deps, authz.Nobody(), resolveNotLinked},
		{"no deps", nil, linked, resolveNotLinked},
		{"nil adapter", (*pb.PBDeps)(nil), linked, resolveNotLinked},
		// The gate passes for a linked caller; with Services unwired the
		// next stop is the fail-soft "unavailable".
		{"linked", deps, linked, resolveUnavailable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			container, res := resolveCallerContainer(c.deps, c.p)
			if res != c.want || container != "" {
				t.Fatalf("resolveCallerContainer = (%q, %d), want (\"\", %d)", container, res, c.want)
			}
		})
	}
}
