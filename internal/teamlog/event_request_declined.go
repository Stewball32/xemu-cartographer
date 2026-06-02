package teamlog

// EventRequestDeclined fires when an owner/manager rejects a join request
// via POST /api/team-membership-requests/{id}/decline. Actor = the
// declining owner/manager; subject_user = the requester.
const EventRequestDeclined Event = "request_declined"

// RequestDeclinedPayload records the request id.
type RequestDeclinedPayload struct {
	RequestID string `json:"request_id"`
}
