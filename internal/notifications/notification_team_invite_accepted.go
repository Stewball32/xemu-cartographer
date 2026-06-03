package notifications

// NotifTypeTeamInviteAccepted is delivered to the inviter when the recipient
// accepts an invite. The link out points at the team page so the inviter can
// see the new roster row materialized.
const NotifTypeTeamInviteAccepted Type = "team_invite_accepted"

// TeamInviteAcceptedPayload mirrors TeamInvitePayload but records the
// acceptor's handle instead of the initiator. RequestID is preserved so the
// notification row can resolve back to the closed request if needed.
type TeamInviteAcceptedPayload struct {
	RequestID      string `json:"request_id"`
	TeamID         string `json:"team_id"`
	TeamName       string `json:"team_name"`
	TeamSlug       string `json:"team_slug"`
	AcceptorID     string `json:"acceptor_id"`
	AcceptorName   string `json:"acceptor_name,omitempty"`
	AcceptorHandle string `json:"acceptor_handle"`
}
