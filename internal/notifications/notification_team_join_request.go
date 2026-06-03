package notifications

// NotifTypeTeamJoinRequest is delivered to each owner/manager of a team when
// a user requests to join. Any one of them can Accept (creates the roster
// row using the requester's default_gamertag) or Decline. Once a request is
// resolved by any owner/manager, the other owners' notification rows still
// exist but the inline buttons go inert (the underlying request row is no
// longer pending).
const NotifTypeTeamJoinRequest Type = "team_join_request"

// TeamJoinRequestPayload carries the team identity + the requester's display
// info. RequestID points at the team_membership_requests row.
type TeamJoinRequestPayload struct {
	RequestID       string `json:"request_id"`
	TeamID          string `json:"team_id"`
	TeamName        string `json:"team_name"`
	TeamSlug        string `json:"team_slug"`
	RequesterID     string `json:"requester_id"`
	RequesterName   string `json:"requester_name,omitempty"`
	RequesterHandle string `json:"requester_handle"`
}
