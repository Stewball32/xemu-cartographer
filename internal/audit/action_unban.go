package audit

// ActionUnban records an admin lifting a ban (both indefinite ActionBan and
// time-bounded ActionTimeout). The hook treats `is_banned: true → false` as
// the trigger regardless of whether banned_until was set.
const ActionUnban Action = "unban"

// UnbanPayload accompanies an ActionUnban audit row. Reason captures the
// admin's justification for early lift; empty when a timeout simply expired
// (in that case no audit row is written — expiry is passive).
type UnbanPayload struct {
	Reason string `json:"reason,omitempty"`
}
