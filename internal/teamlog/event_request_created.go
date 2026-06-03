package teamlog

// EventRequestCreated fires when a user submits a join request via POST
// /api/teams/{teamId}/join-requests. Actor + subject_user are the same
// person (the requester); the team relation identifies the destination.
const EventRequestCreated Event = "request_created"

// RequestCreatedPayload records the request id so the timeline can
// resolve back to the original row.
type RequestCreatedPayload struct {
	RequestID string `json:"request_id"`
}
