package teamlog

// EventMemberLeft fires when a user self-leaves via POST /api/rosters/{id}/leave
// or when the soft-delete cascade in users_soft_delete_pii stamps left_at
// on their active rows. Distinct from EventMemberRemoved, which is an
// owner/manager action against another member.
const EventMemberLeft Event = "member_left"

// MemberLeftPayload identifies the gamertag the user was repping when they
// left. ViaSoftDelete distinguishes a deliberate leave from the cascade.
type MemberLeftPayload struct {
	Gamertag      string `json:"gamertag"`
	ViaSoftDelete bool   `json:"via_soft_delete,omitempty"`
}
