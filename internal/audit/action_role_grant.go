package audit

// ActionRoleGrant records an admin (or the M08 migration backfill) granting a
// role to a user. The target row is the user; the granted role is in the
// payload. Pair with ActionRoleRevoke on the inverse.
const ActionRoleGrant Action = "role_grant"

// RoleGrantPayload accompanies an ActionRoleGrant audit row. ByMigration
// distinguishes "M08 backfill wrote this row" (actor=nil) from a real admin
// grant — the admin-history reader uses it to render a different timeline
// label.
type RoleGrantPayload struct {
	RoleSlug    string `json:"role_slug"`
	ByMigration bool   `json:"by_migration,omitempty"`
}
