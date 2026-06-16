package audit

// ActionRoleRevoke records an admin removing a role from a user. Soft-delete
// cascade rows (M08f) use this same action with actor=nil and Reason set to
// the soft-delete marker.
const ActionRoleRevoke Action = "role_revoke"

// RoleRevokePayload accompanies an ActionRoleRevoke audit row. Reason is
// free-text captured from the admin UI (e.g. "violated CoC §3.2"); empty
// when the revoke is a cascade side-effect.
type RoleRevokePayload struct {
	RoleSlug string `json:"role_slug"`
	Reason   string `json:"reason,omitempty"`
}
