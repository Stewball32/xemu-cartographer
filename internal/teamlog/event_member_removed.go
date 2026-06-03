package teamlog

// EventMemberRemoved fires when an owner/manager kicks another member via
// POST /api/rosters/{id}/remove. The actor relation on the row identifies
// the owner/manager; subject_user is the kicked member.
const EventMemberRemoved Event = "member_removed"

// MemberRemovedPayload is sparse — the actor + subject relations on the
// team_log row carry the who; we just record the gamertag that was active
// on the removed roster row for timeline disambiguation when a user has
// repped a team under multiple handles.
type MemberRemovedPayload struct {
	Gamertag string `json:"gamertag"`
}
