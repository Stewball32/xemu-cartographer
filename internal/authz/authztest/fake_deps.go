// Package authztest provides an in-memory authz.Deps for tests in every
// slice (S1 rule tests, S2 adapter tests, S4 hook tests, S5 WS tests).
package authztest

import (
	"time"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
)

// FakeDeps is a plain struct implementing authz.Deps from exported fields.
// A zero FakeDeps answers everything fail-closed (false / "" / 0 / not
// found) and Now() returns time.Now() until Clock is set.
//
// Map keys:
//
//	Rostered   "userID|instance"
//	Authority  "userID|teamID"
//	Members    "userID|teamID"
//	Owners     "collection/id"
//	Holds      "userID|slug"
//	Tokens     kid
type FakeDeps struct {
	Rostered   map[string]bool
	Owned      map[string]string // userID → owned box name
	Instances  map[string]bool
	Consoles   map[string]string // sanitized console name → instance
	Authority  map[string]bool
	Members    map[string]bool
	Owners     map[string]string
	Levels     map[string]int // role slug → level
	Admins     int
	Holds      map[string]bool // user holds role slug; the map is definitive (absent = known not to hold)
	AnonScopes []string
	Tokens     map[string]authz.TokenRow
	Clock      time.Time

	// Optional overrides; when set they win over the maps above.
	NowFunc               func() time.Time
	RosteredInFunc        func(userID, instance string) bool
	OwnedBoxFunc          func(userID string) string
	InstanceExistsFunc    func(name string) bool
	InstanceByConsoleFunc func(consoleName string) string
	TeamAuthorityFunc     func(userID, teamID string) bool
	ActiveMemberFunc      func(userID, teamID string) bool
	RecordOwnerFunc       func(collection, id string) string
	RoleLevelFunc         func(slug string) (int, bool)
	AdminCountFunc        func() int
	HoldsRoleFunc         func(userID, slug string) (holds, ok bool)
	AnonymousScopesFunc   func() []string
	LookupTokenFunc       func(kid string) (authz.TokenRow, bool)
}

var _ authz.Deps = (*FakeDeps)(nil)

// Key builds the "a|b" key used by Rostered, Authority, Members and Holds.
func Key(a, b string) string { return a + "|" + b }

// OwnerKey builds the "collection/id" key used by Owners.
func OwnerKey(collection, id string) string { return collection + "/" + id }

// Now returns Clock when set, else the wall clock.
func (f *FakeDeps) Now() time.Time {
	if f.NowFunc != nil {
		return f.NowFunc()
	}
	if !f.Clock.IsZero() {
		return f.Clock
	}
	return time.Now()
}

func (f *FakeDeps) RosteredIn(userID, instance string) bool {
	if f.RosteredInFunc != nil {
		return f.RosteredInFunc(userID, instance)
	}
	return f.Rostered[Key(userID, instance)]
}

func (f *FakeDeps) OwnedBox(userID string) string {
	if f.OwnedBoxFunc != nil {
		return f.OwnedBoxFunc(userID)
	}
	return f.Owned[userID]
}

func (f *FakeDeps) InstanceExists(name string) bool {
	if f.InstanceExistsFunc != nil {
		return f.InstanceExistsFunc(name)
	}
	return f.Instances[name]
}

func (f *FakeDeps) InstanceByConsole(consoleName string) string {
	if f.InstanceByConsoleFunc != nil {
		return f.InstanceByConsoleFunc(consoleName)
	}
	return f.Consoles[consoleName]
}

func (f *FakeDeps) TeamAuthority(userID, teamID string) bool {
	if f.TeamAuthorityFunc != nil {
		return f.TeamAuthorityFunc(userID, teamID)
	}
	return f.Authority[Key(userID, teamID)]
}

func (f *FakeDeps) ActiveMember(userID, teamID string) bool {
	if f.ActiveMemberFunc != nil {
		return f.ActiveMemberFunc(userID, teamID)
	}
	return f.Members[Key(userID, teamID)]
}

func (f *FakeDeps) RecordOwner(collection, id string) string {
	if f.RecordOwnerFunc != nil {
		return f.RecordOwnerFunc(collection, id)
	}
	return f.Owners[OwnerKey(collection, id)]
}

func (f *FakeDeps) RoleLevel(slug string) (int, bool) {
	if f.RoleLevelFunc != nil {
		return f.RoleLevelFunc(slug)
	}
	lvl, ok := f.Levels[slug]
	return lvl, ok
}

func (f *FakeDeps) AdminCount() int {
	if f.AdminCountFunc != nil {
		return f.AdminCountFunc()
	}
	return f.Admins
}

// HoldsRole answers from Holds as a definitive lookup (ok=true either way);
// set HoldsRoleFunc to report an unanswerable one (ok=false).
func (f *FakeDeps) HoldsRole(userID, slug string) (holds, ok bool) {
	if f.HoldsRoleFunc != nil {
		return f.HoldsRoleFunc(userID, slug)
	}
	return f.Holds[Key(userID, slug)], true
}

// AnonymousScopes returns a copy so callers cannot mutate the fake's row.
func (f *FakeDeps) AnonymousScopes() []string {
	if f.AnonymousScopesFunc != nil {
		return f.AnonymousScopesFunc()
	}
	out := make([]string, len(f.AnonScopes))
	copy(out, f.AnonScopes)
	return out
}

func (f *FakeDeps) LookupToken(kid string) (authz.TokenRow, bool) {
	if f.LookupTokenFunc != nil {
		return f.LookupTokenFunc(kid)
	}
	row, ok := f.Tokens[kid]
	return row, ok
}
