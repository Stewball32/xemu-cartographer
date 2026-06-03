package teamlog

// EventTeamRenamed fires when the teams.name field changes (owner edit or
// admin override). Mirrors the M22d audit_log ActionRename row — the
// duplicate is intentional: audit_log keeps its admin-only consumer,
// team_log feeds the public team page's "formerly known as" reader.
const EventTeamRenamed Event = "team_renamed"

// TeamRenamedPayload records the before/after values and the actor type so
// the timeline can render "owner renamed" vs "admin renamed" without an
// extra join.
type TeamRenamedPayload struct {
	PrevName string `json:"prev_name"`
	NewName  string `json:"new_name"`
	ByAdmin  bool   `json:"by_admin,omitempty"`
}
