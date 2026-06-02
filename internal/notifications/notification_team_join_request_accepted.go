package notifications

// NotifTypeTeamJoinRequestAccepted is delivered to the requester when an
// owner/manager accepts their join request. The roster row is live; the
// link out points at the team page.
const NotifTypeTeamJoinRequestAccepted Type = "team_join_request_accepted"

// TeamJoinRequestAcceptedPayload mirrors TeamInviteAcceptedPayload from the
// other direction: who accepted (acceptor = an owner/manager) and on which
// team.
type TeamJoinRequestAcceptedPayload struct {
	RequestID      string `json:"request_id"`
	TeamID         string `json:"team_id"`
	TeamName       string `json:"team_name"`
	TeamSlug       string `json:"team_slug"`
	AcceptorID     string `json:"acceptor_id"`
	AcceptorName   string `json:"acceptor_name,omitempty"`
	AcceptorHandle string `json:"acceptor_handle"`
}
