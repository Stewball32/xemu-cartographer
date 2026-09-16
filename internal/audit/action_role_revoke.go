package audit

// ActionRoleRevoke records an admin removing a role from a user. Soft-delete
// cascade rows (M08f) use this same action with actor=nil and Reason set to
// the soft-delete marker.
const ActionRoleRevoke Action = "role_revoke"

// RoleRevokePayload accompanies an ActionRoleRevoke audit row. Reason is
// free-text captured from the admin UI (e.g. "violated CoC §3.2"); empty
// when the revoke is a cascade side-effect. BySuperuser / Actor mirror
// RoleGrantPayload (PD-14, A.9).
type RoleRevokePayload struct {
	RoleSlug    string `json:"role_slug"`
	Reason      string `json:"reason,omitempty"`
	BySuperuser bool   `json:"by_superuser,omitempty"`
	Actor       string `json:"actor,omitempty"`
}
