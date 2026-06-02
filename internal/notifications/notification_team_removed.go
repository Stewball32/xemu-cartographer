package notifications

// NotifTypeTeamRemoved is delivered to a user when an owner/manager removes
// them from a team roster (sets left_at on their roster row from another
// user's action — distinct from a self-leave, which is silent). The
// recipient can still see the team page; the link out points there.
const NotifTypeTeamRemoved Type = "team_removed"

// TeamRemovedPayload carries the team identity for context, the gamertag
// that was on the removed roster row (a user may rep different teams under
// different handles, so this disambiguates), and the actor who triggered
// the removal.
type TeamRemovedPayload struct {
	RosterID        string `json:"roster_id"`
	TeamID          string `json:"team_id"`
	TeamName        string `json:"team_name"`
	TeamSlug        string `json:"team_slug"`
	RemovedGamertag string `json:"removed_gamertag"`
	RemovedByID     string `json:"removed_by_id"`
	RemovedByName   string `json:"removed_by_name,omitempty"`
	RemovedByHandle string `json:"removed_by_handle"`
}
