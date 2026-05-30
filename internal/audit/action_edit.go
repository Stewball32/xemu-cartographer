package audit

// ActionEdit records an owner-driven change to a moderated field that
// automatically downgrades the row's status for re-check. The canonical
// case: an owner edits gamertags.tag of an approved row — the
// gamertags_status_transitions hook flips status approved→allowed and
// writes an ActionEdit row so the trail shows why the row left the
// approved state without an admin involved.
//
// Distinct from a plain PB record update (which doesn't audit) because
// the moderation-bearing edit changes the row's trust posture. Admin
// edits to non-moderated fields (e.g. corrections via /_/) are not
// audited via ActionEdit; they don't flip status.
const ActionEdit Action = "edit"

// EditPayload accompanies an ActionEdit audit row. PrevTag / NewTag capture
// the before/after of the moderated field; PrevStatus / NewStatus capture
// the resulting status transition (typically approved → allowed).
type EditPayload struct {
	Field      string `json:"field"`
	PrevValue  string `json:"prev_value,omitempty"`
	NewValue   string `json:"new_value,omitempty"`
	PrevStatus string `json:"prev_status,omitempty"`
	NewStatus  string `json:"new_status,omitempty"`
}
