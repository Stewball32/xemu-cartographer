package teamlog

// EventInviteCreated fires when an owner/manager opens a new invite via
// POST /api/teams/{teamId}/invite. The actor relation identifies the
// inviter; subject_user is the invitee.
const EventInviteCreated Event = "invite_created"

// InviteCreatedPayload records the request id so the timeline can deep-link
// back to the request row even after it resolves.
type InviteCreatedPayload struct {
	RequestID string `json:"request_id"`
}
