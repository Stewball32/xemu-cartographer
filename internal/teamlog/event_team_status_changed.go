package teamlog

// EventTeamStatusChanged fires on any teams.status transition (approve /
// block / unblock / auto-downgrade-on-rename). audit_log already captures
// the moderation flavor through M22d's ActionBlock/Unblock/Approve/Edit
// rows; this duplicate write puts the same transition on the team's
// activity timeline.
const EventTeamStatusChanged Event = "team_status_changed"

// TeamStatusChangedPayload mirrors the audit payload shape for status
// transitions (PrevStatus + NewStatus) so a single render path on the
// team page can handle both.
type TeamStatusChangedPayload struct {
	PrevStatus string `json:"prev_status"`
	NewStatus  string `json:"new_status"`
}
