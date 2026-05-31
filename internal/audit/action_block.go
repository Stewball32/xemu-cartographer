package audit

// ActionBlock records an admin (or system, during migration backfill) blocking
// a target row. Currently scoped to gamertags + teams; future actions like
// banning a user use ActionBan / ActionTimeout instead since the moderation
// model + payload shape differ.
const ActionBlock Action = "block"

// BlockPayload accompanies an ActionBlock audit row. Reason is free-text
// captured from the admin queue UI in M22b+. PrevStatus lets a reader
// reconstruct the transition without joining back to the row's previous
// audit_log entry — empty when the row was never in another moderated state
// (e.g. M22a-era pre-migration synthetic backfill rows).
type BlockPayload struct {
	Reason     string `json:"reason,omitempty"`
	PrevStatus string `json:"prev_status,omitempty"`
}
