package notifications

// NotifTypeTeamInvite is delivered to a user when a team owner/manager has
// invited them. The recipient can Accept (creates a roster row using their
// default_gamertag) or Decline from the bell dropdown / /notifications/ page.
const NotifTypeTeamInvite Type = "team_invite"

// TeamInvitePayload carries the team identity for the link-out + the
// initiator's display info for the row copy. RequestID points at the
// team_membership_requests row that the accept/decline routes act on.
type TeamInvitePayload struct {
	RequestID         string `json:"request_id"`
	TeamID            string `json:"team_id"`
	TeamName          string `json:"team_name"`
	TeamSlug          string `json:"team_slug"`
	InitiatedByID     string `json:"initiated_by_id"`
	InitiatedByName   string `json:"initiated_by_name,omitempty"`
	InitiatedByHandle string `json:"initiated_by_handle"`
}
