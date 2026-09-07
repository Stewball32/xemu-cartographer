package handlers

import (
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/authz/authztest"
)

// seedAnonScopes are the scopes the seeded anonymous role carries — the
// console door's whole grant (PD-1).
func seedAnonScopes(t *testing.T) []string {
	t.Helper()
	role, ok := authz.SeedRole("anonymous")
	if !ok {
		t.Fatal("no seeded anonymous role")
	}
	return role.Scopes
}

// TestJoinRoom_Can_anonymous_host: the tokenless console door. An
// anonymous connection bound to a live console (?console=<name> matched
// a runner) may join exactly the viewer-safe class rooms of that one
// instance that the anonymous role names — never the bare room (the
// unfiltered legacy replay), never another instance's rooms, never the
// aggregate feeds — and the admin room is not a host room, so the door
// cannot be ridden into it.
func TestJoinRoom_Can_anonymous_host(t *testing.T) {
	scopes := seedAnonScopes(t)
	deps := &authztest.FakeDeps{AnonScopes: scopes}
	bound := authz.Anonymous("pod-a", scopes)
	runJoinCases(t, deps, []joinCase{
		{"bound game_filtered", bound, "host:pod-a:game_filtered", true},
		{"bound tick", bound, "host:pod-a:tick", true},
		{"bound scenario", bound, "host:pod-a:scenario", true},
		{"bound event_filtered", bound, "host:pod-a:event_filtered", true},
		{"bound unfiltered game denied", bound, "host:pod-a:game", false},
		{"bound unfiltered event denied", bound, "host:pod-a:event", false},
		{"bound objects denied", bound, "host:pod-a:objects", false},
		{"bound debug denied", bound, "host:pod-a:debug", false},
		{"bound bare room denied", bound, "host:pod-a", false},
		{"bound other instance denied", bound, "host:pod-b:tick", false},
		{"bound summary denied", bound, "host:summary", false},
		{"bound legacy all denied", bound, "host:all", false},
		{"bound admin room denied", bound, "admin:dashboard", false},
	})
}

// TestJoinRoom_Can_anonymous_unbound: without a binding — no ?console=,
// or one that matched no live runner — the same scopes open nothing under
// host:, and the scopeless Nobody() opens nothing even when bound. A wide
// room.join:* (which no seed grants anonymous) is still held to the
// binding.
func TestJoinRoom_Can_anonymous_unbound(t *testing.T) {
	scopes := seedAnonScopes(t)
	deps := &authztest.FakeDeps{AnonScopes: scopes}
	unbound := authz.Anonymous("", scopes)
	scopelessBound := authz.Principal{Kind: authz.KindAnonymous, Bound: map[string]string{"instance": "pod-a"}}
	wideBound := authz.Anonymous("pod-a", []string{"room.join:*"})
	runJoinCases(t, deps, []joinCase{
		{"unbound tick", unbound, "host:pod-a:tick", false},
		{"unbound game_filtered", unbound, "host:pod-a:game_filtered", false},
		{"unbound bare", unbound, "host:pod-a", false},
		{"unbound summary", unbound, "host:summary", false},
		{"nobody tick", authz.Nobody(), "host:pod-a:tick", false},
		{"nobody bare", authz.Nobody(), "host:pod-a", false},
		{"nobody summary", authz.Nobody(), "host:summary", false},
		{"scopeless but bound", scopelessBound, "host:pod-a:tick", false},
		{"wide scope own instance", wideBound, "host:pod-a:tick", true},
		{"wide scope other instance", wideBound, "host:pod-b:tick", false},
		{"wide scope bare denied", wideBound, "host:pod-a", false},
		{"wide scope summary denied", wideBound, "host:summary", false},
	})
}
