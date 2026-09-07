package authz

import "time"

// Deps is everything Can needs from the outside world. internal/authz/pb
// implements it over the live PocketBase app, scraper manager and
// provisioner; authztest.FakeDeps is the in-memory version for tests.
//
// Every method must be safe to call with empty arguments and must answer
// fail-closed (false / "" / 0 / not found) on any lookup error. HoldsRole is
// the one answer a Deny predicate reads, where "false" would open the gate:
// it reports ok=false for a lookup it could not complete so the rule can
// refuse instead.
type Deps interface {
	Now() time.Time

	// membership / identity
	RosteredIn(userID, instance string) bool     // usable gamertags ∩ Membership() with rostergrace TTL (A.2)
	OwnedBox(userID string) string               // "<prefix>play-<uid>" or "" when provisioner absent
	InstanceExists(name string) bool             // a scraper runner / container by that name is known
	InstanceByConsole(consoleName string) string // instance whose sanitized XboxName == sanitized name, else ""

	// teams
	TeamAuthority(userID, teamID string) bool // active owner/manager row, or created_by with no owner row (A.8)
	ActiveMember(userID, teamID string) bool  // active roster row
	RecordOwner(collection, id string) string // owning users.id or ""

	// roles
	RoleLevel(slug string) (level int, ok bool)     // roles.level by slug; ok=false for an unknown slug
	AdminCount() int                                // users holding admin (excluding soft-deleted)
	HoldsRole(userID, slug string) (holds, ok bool) // a user_roles row for (userID, roles.slug) exists; ok=false when the lookup failed (holds unknown)
	AnonymousScopes() []string                      // roles row "anonymous".scopes

	// tokens
	LookupToken(kid string) (TokenRow, bool) // api_tokens row (or the in-memory legacy-env row)
}

// TokenRow is the Deps-side view of an api_tokens record (plus the in-memory
// legacy-env row). KeyHash is the sha256 hex of the secret; the secret itself
// is never stored.
type TokenRow struct {
	Kid, KeyHash, Kind, Label, UserID, Container, StationID string
	Scopes                                                  []string
	Gamertags                                               []string  // sanitized tags of api_tokens.gamertags (PD-8); empty = unbound station
	ExpiresAt                                               time.Time // zero = never
	Revoked                                                 bool
}
