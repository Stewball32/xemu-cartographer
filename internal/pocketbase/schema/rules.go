package schema

// rules.go centralizes the PB-rule string shapes that the M22-era schemas
// used to express via the bare `@request.auth.isAdmin = true` check. The
// M08 cutover swapped to the user_roles subquery form documented below;
// keeping the string in one place means a typo at the rule layer is one fix,
// not eight.
//
// The `?=` (any-match) operator is required: `@collection.user_roles`
// resolves to a list, and `=` would evaluate as "all rows match" which is
// the wrong semantic for "does at least one row exist."
//
// Smoke-tested against PB v0.36.7 in the M08 8-pre spike on 2026-06-03:
// the rule saves cleanly, evaluates live (deleting a user_roles row
// immediately removes admin access on the next request), and works in
// composed rules when wrapped in parens for precedence.
//
// Read sites (Go code) consume `internal/roles.Has(app, userID, slug)`
// instead — keep the two surfaces in sync when adding role slugs that need
// PB-rule gating.

// hasAdminRole is the M08 successor to `@request.auth.isAdmin = true`.
// Matches when the authenticated user holds a user_roles row pointing at
// the admin role. Wrap in parens when composing with `||` / `&&` so
// operator precedence stays predictable.
const hasAdminRole = `@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= "admin"`

// hasOrganizerRole matches when the authenticated user holds a user_roles row
// pointing at the organizer role. Same shape/caveats as hasAdminRole.
const hasOrganizerRole = `@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= "organizer"`

// organizerOrAdmin is the mutate gate for the shared gametype library and the
// game (XBE) uploads in the gamertag identity system: either role passes.
// Already parenthesised, so it composes directly into a larger rule.
const organizerOrAdmin = `((` + hasOrganizerRole + `) || (` + hasAdminRole + `))`
