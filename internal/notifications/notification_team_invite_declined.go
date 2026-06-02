package notifications

// NotifTypeTeamInviteDeclined is delivered to the inviter when the recipient
// declines. The request is closed; no further action is required of either
// party. The inviter can retry by sending a new invite.
const NotifTypeTeamInviteDeclined Type = "team_invite_declined"

// TeamInviteDeclinedPayload carries the team identity for context and the
// declining user's handle so the inviter knows who said no.
type TeamInviteDeclinedPayload struct {
	RequestID      string `json:"request_id"`
	TeamID         string `json:"team_id"`
	TeamName       string `json:"team_name"`
	TeamSlug       string `json:"team_slug"`
	DeclinerID     string `json:"decliner_id"`
	DeclinerName   string `json:"decliner_name,omitempty"`
	DeclinerHandle string `json:"decliner_handle"`
}
