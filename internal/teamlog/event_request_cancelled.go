package teamlog

// EventRequestCancelled fires when the requester (or the soft-delete
// cascade) withdraws a pending join request. Subject_user is the
// requester; actor is the canceller (same person on user-initiated
// cancel, nil on the cascade path).
const EventRequestCancelled Event = "request_cancelled"

// RequestCancelledPayload records the request id and whether the cancel
// came from the soft-delete cascade.
type RequestCancelledPayload struct {
	RequestID     string `json:"request_id"`
	ViaSoftDelete bool   `json:"via_soft_delete,omitempty"`
}
