package notifications

// NotifTypeTeamInviteCancelled is delivered to the recipient when the
// inviter (or a soft-delete cascade) cancels a pending invite before they
// could act on it. The request row is closed; the recipient has nothing to
// do.
const NotifTypeTeamInviteCancelled Type = "team_invite_cancelled"

// TeamInviteCancelledPayload carries the team identity for context. The
// canceller is identified by handle so the row can render "$inviter
// cancelled their invite to $team".
type TeamInviteCancelledPayload struct {
	RequestID       string `json:"request_id"`
	TeamID          string `json:"team_id"`
	TeamName        string `json:"team_name"`
	TeamSlug        string `json:"team_slug"`
	CancelledByID   string `json:"cancelled_by_id"`
	CancelledByName string `json:"cancelled_by_name,omitempty"`
	CancelledHandle string `json:"cancelled_handle,omitempty"`
	BySoftDelete    bool   `json:"by_soft_delete,omitempty"`
}
