package teamlog

// EventMemberAdded fires when a roster row materializes — either from an
// owner/manager accepting a join request or from a user accepting an
// invite. The Via field disambiguates the two paths so a UI render can
// show "Jane joined via invite from Bob" vs "Jane joined via her request".
//
// We do NOT also write a separate invite_accepted / request_accepted
// event. The roster row's existence is the canonical "they joined" signal;
// a paired event would just be redundant noise on the timeline.
const EventMemberAdded Event = "member_added"

// MemberAddedPayload carries the route the membership took plus the
// originating request id so the timeline can deep-link back to the closed
// request for audit purposes.
type MemberAddedPayload struct {
	Via       string `json:"via"` // "invite" | "request"
	RequestID string `json:"request_id"`
	Gamertag  string `json:"gamertag"`
}
