package audit

// ActionRename records a change to a target row's user-facing identifier
// (gamertags.tag is the obvious case; teams.name is the M22d driver). Distinct
// from ActionEdit: ActionEdit fires only on the auto-downgrade case where a
// moderated-field change also flips status, while ActionRename is the always-on
// audit for the rename itself. A single owner-driven rename of an approved
// team produces two audit rows — one ActionRename + one ActionEdit — so the
// team page "formerly known as" reader can filter on ActionRename and the
// moderation queue can filter on ActionEdit without overlap.
const ActionRename Action = "rename"

// RenamePayload accompanies an ActionRename audit row. ByAdmin distinguishes
// "admin renamed someone's row" from the more common "owner renamed their own
// row" so the team-page history view can show context (admin renames are rare
// and worth flagging).
type RenamePayload struct {
	PrevName string `json:"prev_name"`
	NewName  string `json:"new_name"`
	ByAdmin  bool   `json:"by_admin,omitempty"`
}
