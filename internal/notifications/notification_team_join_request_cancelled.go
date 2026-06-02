package notifications

// NotifTypeTeamJoinRequestCancelled is delivered to each owner/manager of
// the team when the requester (or a soft-delete cascade) cancels a pending
// join request. The owners' notification rows for the original request stay
// in their list as "this is no longer actionable" markers.
const NotifTypeTeamJoinRequestCancelled Type = "team_join_request_cancelled"

// TeamJoinRequestCancelledPayload carries the team identity for context.
// The canceller is identified by handle; BySoftDelete distinguishes a
// user-initiated cancel from the cascade triggered by account deletion.
type TeamJoinRequestCancelledPayload struct {
	RequestID       string `json:"request_id"`
	TeamID          string `json:"team_id"`
	TeamName        string `json:"team_name"`
	TeamSlug        string `json:"team_slug"`
	CancelledByID   string `json:"cancelled_by_id"`
	CancelledByName string `json:"cancelled_by_name,omitempty"`
	CancelledHandle string `json:"cancelled_handle,omitempty"`
	BySoftDelete    bool   `json:"by_soft_delete,omitempty"`
}
