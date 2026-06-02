package teamlog

// EventInviteDeclined fires when the invitee declines via POST
// /api/team-membership-requests/{id}/decline. Actor = decliner;
// subject_user = decliner (same person on the receiving side of the
// invite, redundant but kept for symmetry with other events).
const EventInviteDeclined Event = "invite_declined"

// InviteDeclinedPayload is the request id only — the actor + subject
// columns carry who declined.
type InviteDeclinedPayload struct {
	RequestID string `json:"request_id"`
}
