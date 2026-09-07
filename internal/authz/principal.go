package authz

import "sort"

// Kind is the principal class a resolver produced. Every rule in rules.go
// admits an explicit set of kinds; superuser and internal are allow-all
// (after the Deny predicates) and never appear in a rule's kind set.
type Kind string

const (
	KindPBUser    Kind = "pb_user"
	KindSuperuser Kind = "superuser"
	KindMachine   Kind = "machine"
	KindSpectator Kind = "spectator"
	KindDevice    Kind = "device"
	KindAnonymous Kind = "anonymous"
	KindDiscord   Kind = "discord"
	KindInternal  Kind = "internal"
)

// allKinds is the closed set of principal kinds (rules_test pins the rule
// table's kind sets against it).
var allKinds = []Kind{
	KindPBUser,
	KindSuperuser,
	KindMachine,
	KindSpectator,
	KindDevice,
	KindAnonymous,
	KindDiscord,
	KindInternal,
}

// Kinds returns the closed set of principal kinds (a fresh slice).
func Kinds() []Kind {
	out := make([]Kind, len(allKinds))
	copy(out, allKinds)
	return out
}

// Known reports whether k is one of the closed set of kinds.
func (k Kind) Known() bool {
	for _, known := range allKinds {
		if k == known {
			return true
		}
	}
	return false
}

// Principal is who is asking. It is a value: resolvers build a fresh one per
// request / per WS re-resolve tick and never mutate a stored one. Scopes MUST
// be canonical (CanonScopes) before storing — Match is strict and rejects
// upper-case or padded patterns rather than normalising them.
//
// Field meaning per kind (design §1):
//
//	pb_user    ID = users.id, UserID = same, Collection = "users", Roles, Scopes, Level from user_roles ∪ roles
//	superuser  ID = _superusers.id, Collection = "_superusers", no scopes (allow-all)
//	machine    ID = kid, UserID optional, Scopes from api_tokens; Extra station_id / label
//	spectator  ID = kid, Bound["instance"] = the one instance its scopes name
//	device     ID = kid, Bound["instance"] = api_tokens.container
//	anonymous  ID = "", Scopes = the anonymous role row, Bound["instance"] when ?console= resolved
//	discord    ID = member snowflake, UserID = linked users.id or ""; Extra guild_id / member_permissions
//	internal   ID = actor label (allow-all; bypasses last_admin with a WARN in the adapter)
type Principal struct {
	Kind       Kind
	ID         string            // see the table above
	UserID     string            // users.id when known, else ""
	Collection string            // "users" | "_superusers" | "" — audit actor eligibility
	Roles      []string          // role slugs (pb_user only), sorted
	Scopes     []string          // canonical (Canon), deduped, sorted
	Level      int               // max roles.level (pb_user); 0 otherwise
	Bound      map[string]string // "instance" for anonymous/spectator/device
	Extra      map[string]string // "station_id","label","guild_id","member_permissions","token_user","gamertags"
}

// Is reports whether the principal is of kind k.
func (p Principal) Is(k Kind) bool { return p.Kind == k }

// HasRole reports whether the principal holds the role slug (pb_user only —
// every other kind carries no roles).
func (p Principal) HasRole(slug string) bool {
	if slug == "" {
		return false
	}
	for _, r := range p.Roles {
		if r == slug {
			return true
		}
	}
	return false
}

// HasScope reports whether any of the principal's scopes matches want
// ("<action>" or "<action>:<selector>", see Match).
func (p Principal) HasScope(want string) bool {
	_, ok := MatchAny(p.Scopes, want)
	return ok
}

// BoundInstance returns Bound["instance"] ("" when unbound).
func (p Principal) BoundInstance() string {
	return p.Bound["instance"]
}

// IsAdmin reports whether the principal is a superuser or holds the admin
// role. Rules never special-case it — the admin role passes wherever the seed
// gives it a scope — but /api/me and the frontend still surface it.
func (p Principal) IsAdmin() bool {
	return p.Kind == KindSuperuser || p.HasRole("admin")
}

// AuditActorID returns UserID when the principal is a users record (the only
// collection the audit_log actor relation accepts), else "".
func (p Principal) AuditActorID() string {
	if p.Collection == "users" {
		return p.UserID
	}
	return ""
}

// Anonymous builds the console-door principal: unauthenticated, bound to
// boundInstance (may be "" ⇒ can join nothing under host:), carrying the
// anonymous role row's scopes (server data, PD-1).
func Anonymous(boundInstance string, scopes []string) Principal {
	p := Principal{Kind: KindAnonymous, Scopes: CanonScopes(scopes)}
	if boundInstance != "" {
		p.Bound = map[string]string{"instance": boundInstance}
	}
	return p
}

// Internal builds the in-process principal (hooks with nil auth, the seeder,
// in-process writers). It is allow-all in Can and bypasses last_admin (§4.4);
// actor is carried for audit/log lines.
func Internal(actor string) Principal {
	return Principal{Kind: KindInternal, ID: actor}
}

// Superuser builds the PocketBase _superusers principal (allow-all in Can,
// audit actor=null + by_superuser:true, PD-14).
func Superuser(id string) Principal {
	return Principal{Kind: KindSuperuser, ID: id, Collection: "_superusers"}
}

// Nobody is the unbound, scopeless anonymous principal: no credential at all.
// It can join public rooms and nothing else.
func Nobody() Principal {
	return Principal{Kind: KindAnonymous}
}

// CanonScopes canonicalises (Canon), drops empties, de-duplicates and sorts a
// scope list — the normal form Principal.Scopes and TokenRow-derived scopes
// must be stored in. Always returns a non-nil slice.
func CanonScopes(scopes []string) []string {
	seen := make(map[string]bool, len(scopes))
	out := make([]string, 0, len(scopes))
	for _, s := range scopes {
		c := Canon(s)
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}
