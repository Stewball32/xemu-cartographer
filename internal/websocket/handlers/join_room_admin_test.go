package handlers

import (
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/authz/authztest"
)

// adminScopes is what the seeded admin role grants (DECISIONS.md): the
// admin room, every host room, and the scraper actions.
var adminScopes = []string{"admin.*", "room.join:*", "scraper.*"}

// TestJoinRoom_Can_superuser_any: a PocketBase superuser needs no scope
// and no roster for any registered room, aggregates and the admin room
// included — but only with an adapter present (nil deps fail closed; see
// TestJoinRoom_NilDepsFailsClosed).
func TestJoinRoom_Can_superuser_any(t *testing.T) {
	su := authz.Superuser("su")
	runJoinCases(t, &authztest.FakeDeps{}, []joinCase{
		{"bare host room", su, "host:pod-a", true},
		{"class room", su, "host:pod-a:debug", true},
		{"summary", su, "host:summary", true},
		{"legacy all", su, "host:all", true},
		{"admin room", su, "admin:dashboard", true},
		{"public room", su, "public:lobby", true},
	})
}

// TestJoinRoom_Can_admin_host: a user holding the admin role's scopes is
// admitted to every host room without any roster or grace involvement
// (the pre-authz admin bypass, now expressed as room.join:*), while the
// per-instance bare room stays closed to a machine key carrying the same
// scopes.
func TestJoinRoom_Can_admin_host(t *testing.T) {
	admin := pbUser("a1", adminScopes...)
	adminKey := machine("k1", adminScopes...)
	runJoinCases(t, &authztest.FakeDeps{}, []joinCase{
		{"admin bare host room", admin, "host:pod-a", true},
		{"admin class room", admin, "host:pod-a:objects", true},
		{"admin summary", admin, "host:summary", true},
		{"admin legacy all", admin, "host:all", true},
		{"admin key class room", adminKey, "host:pod-a:objects", true},
		{"admin key summary", adminKey, "host:summary", true},
		{"admin key bare host room denied", adminKey, "host:pod-a", false},
	})
}

// TestJoinRoom_Can_admin_admin: the admin room wants admin.admin, which
// only a pb_user (or superuser) may present — a member, a machine key
// with the same scopes, and every bound kind stay out.
func TestJoinRoom_Can_admin_admin(t *testing.T) {
	runJoinCases(t, &authztest.FakeDeps{}, []joinCase{
		{"admin role", pbUser("a1", adminScopes...), "admin:dashboard", true},
		{"exact scope", pbUser("a2", "admin.admin"), "admin:dashboard", true},
		{"member", pbUser("u1"), "admin:dashboard", false},
		{"member with room.join:*", pbUser("u2", "room.join:*"), "admin:dashboard", false},
		{"machine with admin scopes", machine("k1", adminScopes...), "admin:dashboard", false},
		{"spectator", boundKey(authz.KindSpectator, "s1", "pod-a", adminScopes...), "admin:dashboard", false},
		{"device", boundKey(authz.KindDevice, "d1", "pod-a", adminScopes...), "admin:dashboard", false},
		{"nobody", authz.Nobody(), "admin:dashboard", false},
	})
}
