package notifications

// NotifTypeTeamJoinRequestDeclined is delivered to the requester when an
// owner/manager declines their join request. The request row is closed; the
// requester can retry by submitting a new request.
const NotifTypeTeamJoinRequestDeclined Type = "team_join_request_declined"

// TeamJoinRequestDeclinedPayload identifies the declining owner/manager so
// the requester knows who said no (relevant when more than one
// owner/manager exists on the team).
type TeamJoinRequestDeclinedPayload struct {
	RequestID      string `json:"request_id"`
	TeamID         string `json:"team_id"`
	TeamName       string `json:"team_name"`
	TeamSlug       string `json:"team_slug"`
	DeclinerID     string `json:"decliner_id"`
	DeclinerName   string `json:"decliner_name,omitempty"`
	DeclinerHandle string `json:"decliner_handle"`
}
