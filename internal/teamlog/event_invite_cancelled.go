package teamlog

// EventInviteCancelled fires when the initiator (or the soft-delete cascade
// for the initiator's account) revokes a pending invite. Subject_user is
// the would-have-been invitee; actor is the canceller.
const EventInviteCancelled Event = "invite_cancelled"

// InviteCancelledPayload records the request id and whether the cancel
// came from the soft-delete cascade or an explicit user action — useful
// for owner-side activity reports that want to filter out churn caused
// by account deletions.
type InviteCancelledPayload struct {
	RequestID     string `json:"request_id"`
	ViaSoftDelete bool   `json:"via_soft_delete,omitempty"`
}
