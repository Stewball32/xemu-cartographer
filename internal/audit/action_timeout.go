package audit

// ActionTimeout records a time-bounded ban — the user can't authenticate
// until banned_until passes. Auth-rule check is passive: a stale
// `is_banned=true` row whose banned_until is in the past admits the user on
// next auth without anyone clearing the flag. The active expiry cron (visible
// is_banned clear) is deferred to M16+.
const ActionTimeout Action = "timeout"

// TimeoutPayload accompanies an ActionTimeout audit row. ExpiresAt is the
// ISO-8601 timestamp written to users.banned_until. Reason mirrors BanPayload.
type TimeoutPayload struct {
	Reason    string `json:"reason,omitempty"`
	ExpiresAt string `json:"expires_at"`
}
