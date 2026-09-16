package authztest

import (
	"testing"
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
)

// TestFakeDepsImplementsDeps is the compile-time assertion plus a smoke check
// that a zero FakeDeps answers everything fail-closed and never panics.
func TestFakeDepsImplementsDeps(t *testing.T) {
	var d authz.Deps = &FakeDeps{}

	if d.RosteredIn("u", "box1") {
		t.Fatal("zero FakeDeps: RosteredIn should be false")
	}
	if got := d.OwnedBox("u"); got != "" {
		t.Fatalf("zero FakeDeps: OwnedBox = %q", got)
	}
	if d.InstanceExists("box1") {
		t.Fatal("zero FakeDeps: InstanceExists should be false")
	}
	if got := d.InstanceByConsole("spartan-1"); got != "" {
		t.Fatalf("zero FakeDeps: InstanceByConsole = %q", got)
	}
	if d.TeamAuthority("u", "t") || d.ActiveMember("u", "t") {
		t.Fatal("zero FakeDeps: team answers should be false")
	}
	if got := d.RecordOwner("teams", "t"); got != "" {
		t.Fatalf("zero FakeDeps: RecordOwner = %q", got)
	}
	if _, ok := d.RoleLevel("admin"); ok {
		t.Fatal("zero FakeDeps: RoleLevel should report unknown")
	}
	if got := d.AdminCount(); got != 0 {
		t.Fatalf("zero FakeDeps: AdminCount = %d", got)
	}
	if holds, ok := d.HoldsRole("u", "admin"); holds || !ok {
		t.Fatalf("zero FakeDeps: HoldsRole = %v,%v, want a definitive false", holds, ok)
	}
	if got := d.AnonymousScopes(); got == nil || len(got) != 0 {
		t.Fatalf("zero FakeDeps: AnonymousScopes = %v, want empty non-nil", got)
	}
	if _, ok := d.LookupToken("sp_x"); ok {
		t.Fatal("zero FakeDeps: LookupToken should miss")
	}
	if d.Now().IsZero() {
		t.Fatal("zero FakeDeps: Now should fall back to the wall clock")
	}
}

func TestFakeDepsMapsAndOverrides(t *testing.T) {
	clock := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	f := &FakeDeps{
		Rostered:  map[string]bool{Key("u1", "box1"): true},
		Owned:     map[string]string{"u1": "play-u1"},
		Instances: map[string]bool{"box1": true},
		Consoles:  map[string]string{"spartan-1": "box1"},
		Authority: map[string]bool{Key("u1", "t1"): true},
		Members:   map[string]bool{Key("u2", "t1"): true},
		Owners:    map[string]string{OwnerKey("teams", "t1"): "u1"},
		Levels:    map[string]int{"admin": 100},
		Admins:    2,
		Holds:     map[string]bool{Key("u1", "admin"): true},
		AnonScopes: []string{
			"room.join:host:*:tick",
		},
		Tokens: map[string]authz.TokenRow{"sp_abc": {Kid: "sp_abc", Kind: "spectator"}},
		Clock:  clock,
	}

	if !f.RosteredIn("u1", "box1") || f.RosteredIn("u1", "box2") {
		t.Fatal("Rostered map not honoured")
	}
	if f.OwnedBox("u1") != "play-u1" {
		t.Fatal("Owned map not honoured")
	}
	if !f.InstanceExists("box1") || f.InstanceByConsole("spartan-1") != "box1" {
		t.Fatal("Instances / Consoles maps not honoured")
	}
	if !f.TeamAuthority("u1", "t1") || f.TeamAuthority("u2", "t1") {
		t.Fatal("Authority map not honoured")
	}
	if !f.ActiveMember("u2", "t1") || f.RecordOwner("teams", "t1") != "u1" {
		t.Fatal("Members / Owners maps not honoured")
	}
	if lvl, ok := f.RoleLevel("admin"); !ok || lvl != 100 {
		t.Fatalf("RoleLevel(admin) = %d,%v", lvl, ok)
	}
	if f.AdminCount() != 2 {
		t.Fatal("Admins not honoured")
	}
	if holds, ok := f.HoldsRole("u1", "admin"); !holds || !ok {
		t.Fatalf("Holds map not honoured: HoldsRole(u1, admin) = %v,%v", holds, ok)
	}
	for _, pair := range [][2]string{{"u2", "admin"}, {"u1", "member"}} {
		if holds, ok := f.HoldsRole(pair[0], pair[1]); holds || !ok {
			t.Fatalf("Holds map not honoured: HoldsRole(%s, %s) = %v,%v, want a definitive false", pair[0], pair[1], holds, ok)
		}
	}
	scopes := f.AnonymousScopes()
	scopes[0] = "mutated"
	if f.AnonScopes[0] != "room.join:host:*:tick" {
		t.Fatal("AnonymousScopes must return a copy")
	}
	if row, ok := f.LookupToken("sp_abc"); !ok || row.Kind != "spectator" {
		t.Fatal("Tokens map not honoured")
	}
	if !f.Now().Equal(clock) {
		t.Fatal("Clock not honoured")
	}

	f.AdminCountFunc = func() int { return 0 }
	if f.AdminCount() != 0 {
		t.Fatal("AdminCountFunc override should win over Admins")
	}
	f.HoldsRoleFunc = func(userID, slug string) (bool, bool) { return userID == "u2", userID != "u3" }
	if holds, ok := f.HoldsRole("u1", "admin"); holds || !ok {
		t.Fatalf("HoldsRoleFunc override should win over Holds: (u1) = %v,%v", holds, ok)
	}
	if holds, ok := f.HoldsRole("u2", "admin"); !holds || !ok {
		t.Fatalf("HoldsRoleFunc override should win over Holds: (u2) = %v,%v", holds, ok)
	}
	if _, ok := f.HoldsRole("u3", "admin"); ok {
		t.Fatal("HoldsRoleFunc must be able to report an unanswerable lookup")
	}
	f.NowFunc = func() time.Time { return clock.Add(time.Hour) }
	if !f.Now().Equal(clock.Add(time.Hour)) {
		t.Fatal("NowFunc override should win over Clock")
	}
}
