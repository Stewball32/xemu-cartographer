package audit

// ActionApprove records an admin promoting a row to the approved state
// — the strongest "this is vetted, not flagged" signal. From the approved
// state, an owner-driven edit to the moderated field (e.g. gamertags.tag)
// auto-downgrades the row back to allowed for re-check (recorded as an
// ActionEdit audit row, not ActionApprove).
const ActionApprove Action = "approve"

// ApprovePayload accompanies an ActionApprove audit row.
type ApprovePayload struct {
	Reason     string `json:"reason,omitempty"`
	PrevStatus string `json:"prev_status,omitempty"`
}
