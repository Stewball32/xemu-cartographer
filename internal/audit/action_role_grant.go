package audit

// ActionRoleGrant records an admin (or the M08 migration backfill) granting a
// role to a user. The target row is the user; the granted role is in the
// payload. Pair with ActionRoleRevoke on the inverse.
const ActionRoleGrant Action = "role_grant"

// RoleGrantPayload accompanies an ActionRoleGrant audit row. ByMigration
// distinguishes "M08 backfill / internal actor wrote this row" (actor=nil)
// from a real admin grant — the admin-history reader uses it to render a
// different timeline label. BySuperuser marks a PB superuser actor (PD-14:
// the actor relation cannot reference _superusers); Actor carries the
// label of any other actor the relation cannot reference
// ("discord:<snowflake>", A.9).
type RoleGrantPayload struct {
	RoleSlug    string `json:"role_slug"`
	Reason      string `json:"reason,omitempty"`
	ByMigration bool   `json:"by_migration,omitempty"`
	BySuperuser bool   `json:"by_superuser,omitempty"`
	Actor       string `json:"actor,omitempty"`
}
