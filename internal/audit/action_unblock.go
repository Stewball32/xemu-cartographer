package audit

// ActionUnblock records an admin reverting a blocked row back to the allowed
// state. Used for both gamertags and teams; the target_collection on the
// audit_log row disambiguates.
const ActionUnblock Action = "unblock"

// UnblockPayload accompanies an ActionUnblock audit row. Reason captures the
// admin's justification (e.g. "false positive on M22e pre-list").
type UnblockPayload struct {
	Reason string `json:"reason,omitempty"`
}
