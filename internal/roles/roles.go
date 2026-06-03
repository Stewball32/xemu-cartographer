// Package roles owns the read + write paths for the M08 roles + user_roles
// collections. Callers (middleware, route handlers, hooks, /api/me) go
// through Has / Grant / Revoke / Slugs rather than touching the join table
// directly, so the audit_log write + idempotency guarantees stay in one
// place.
//
// Why not a guards/interfaces interface? Same reasoning as internal/teamperms
// (M23c) — the scraper + Discord subsystems don't need role checks (their
// domain is per-instance + per-channel state), so adding a Service interface
// would couple unrelated systems. Direct core.App calls keep the dependency
// one-directional: middleware + route handlers + hooks import this package;
// nothing here reaches outward.
package roles

import (
	"fmt"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/audit"
)

// Has returns true when userID holds an active user_roles row pointing at the
// role with the given slug. Empty userID or empty slug returns (false, nil)
// so the guards can compose without nil-checks at every call site.
func Has(app core.App, userID, slug string) (bool, error) {
	if userID == "" || slug == "" {
		return false, nil
	}
	rows, err := app.FindRecordsByFilter(
		"user_roles",
		"user = {:userID} && role.slug = {:slug}",
		"",
		1, 0,
		dbx.Params{"userID": userID, "slug": slug},
	)
	if err != nil {
		return false, fmt.Errorf("roles.Has(%s, %s): %w", userID, slug, err)
	}
	return len(rows) > 0, nil
}

// Slugs returns every role slug currently held by userID. Used by /api/me to
// hydrate the frontend roles[] and by the admin UI's assignment view. Empty
// userID returns (nil, nil).
//
// Order is unspecified — callers that need a stable order (display, diff)
// should sort client-side.
func Slugs(app core.App, userID string) ([]string, error) {
	if userID == "" {
		return nil, nil
	}
	rows, err := app.FindRecordsByFilter(
		"user_roles",
		"user = {:userID}",
		"",
		0, 0,
		dbx.Params{"userID": userID},
	)
	if err != nil {
		return nil, fmt.Errorf("roles.Slugs(%s): %w", userID, err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	for _, expandErr := range app.ExpandRecords(rows, []string{"role"}, nil) {
		if expandErr != nil {
			return nil, fmt.Errorf("roles.Slugs(%s): expand role: %w", userID, expandErr)
		}
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		role := r.ExpandedOne("role")
		if role == nil {
			continue
		}
		if slug := role.GetString("slug"); slug != "" {
			out = append(out, slug)
		}
	}
	return out, nil
}

// Grant creates a user_roles row pointing at the role with the given slug and
// writes a matching ActionRoleGrant audit row. Idempotent: a duplicate grant
// is a silent no-op (no audit row, no error). grantedBy may be nil — the M08
// migration backfill (8b) and the default-role hook both pass nil so the row
// records "system" provenance.
func Grant(app core.App, userID, slug string, grantedBy *core.Record) error {
	if userID == "" {
		return fmt.Errorf("roles.Grant: userID is required")
	}
	if slug == "" {
		return fmt.Errorf("roles.Grant: slug is required")
	}

	role, err := app.FindFirstRecordByData("roles", "slug", slug)
	if err != nil || role == nil {
		return fmt.Errorf("roles.Grant: lookup role slug=%q: %w", slug, err)
	}

	existing, err := app.FindFirstRecordByFilter(
		"user_roles",
		"user = {:userID} && role = {:roleID}",
		dbx.Params{"userID": userID, "roleID": role.Id},
	)
	if err == nil && existing != nil {
		return nil
	}

	user, err := app.FindRecordById("users", userID)
	if err != nil {
		return fmt.Errorf("roles.Grant: lookup user %s: %w", userID, err)
	}

	col, err := app.FindCollectionByNameOrId("user_roles")
	if err != nil {
		return fmt.Errorf("roles.Grant: lookup user_roles collection: %w", err)
	}
	row := core.NewRecord(col)
	row.Set("user", userID)
	row.Set("role", role.Id)
	if grantedBy != nil {
		row.Set("granted_by", grantedBy.Id)
	}
	if err := app.Save(row); err != nil {
		return fmt.Errorf("roles.Grant: save user_roles for (user=%s role=%s): %w", userID, slug, err)
	}

	byMigration := grantedBy == nil
	if err := audit.Write(app, grantedBy, audit.ActionRoleGrant, user, audit.RoleGrantPayload{
		RoleSlug:    slug,
		ByMigration: byMigration,
	}); err != nil {
		return fmt.Errorf("roles.Grant: audit write: %w", err)
	}
	return nil
}

// Revoke deletes the user_roles row at (userID, slug) and writes a matching
// ActionRoleRevoke audit row. No-op (no error, no audit) when no row exists.
// by may be nil for cascade-driven revokes (e.g. the M8f soft-delete cascade
// clearing every role from a tombstoned user); reason carries the admin's
// justification when present.
func Revoke(app core.App, userID, slug string, by *core.Record, reason string) error {
	if userID == "" {
		return fmt.Errorf("roles.Revoke: userID is required")
	}
	if slug == "" {
		return fmt.Errorf("roles.Revoke: slug is required")
	}

	role, err := app.FindFirstRecordByData("roles", "slug", slug)
	if err != nil || role == nil {
		return fmt.Errorf("roles.Revoke: lookup role slug=%q: %w", slug, err)
	}

	row, err := app.FindFirstRecordByFilter(
		"user_roles",
		"user = {:userID} && role = {:roleID}",
		dbx.Params{"userID": userID, "roleID": role.Id},
	)
	if err != nil || row == nil {
		return nil
	}

	if err := app.Delete(row); err != nil {
		return fmt.Errorf("roles.Revoke: delete user_roles for (user=%s role=%s): %w", userID, slug, err)
	}

	user, err := app.FindRecordById("users", userID)
	if err != nil {
		return fmt.Errorf("roles.Revoke: lookup user %s: %w", userID, err)
	}
	if err := audit.Write(app, by, audit.ActionRoleRevoke, user, audit.RoleRevokePayload{
		RoleSlug: slug,
		Reason:   reason,
	}); err != nil {
		return fmt.Errorf("roles.Revoke: audit write: %w", err)
	}
	return nil
}

// IsAdminAuth is the shorthand the M22-era hooks need: "is this request's
// auth record either a PB superuser or a holder of the admin role?" Returns
// false when auth is nil. Swallows lookup errors and returns false — the
// caller is using this for a gating decision where a transient DB error
// should fail closed.
func IsAdminAuth(app core.App, auth *core.Record) bool {
	if auth == nil {
		return false
	}
	if auth.IsSuperuser() {
		return true
	}
	ok, err := Has(app, auth.Id, "admin")
	if err != nil {
		return false
	}
	return ok
}
