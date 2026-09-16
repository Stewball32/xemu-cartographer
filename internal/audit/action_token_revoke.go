package audit

// ActionTokenRevoke records an api_tokens row being revoked. The target is
// the api_tokens row (kept, flagged revoked — never deleted — so the audit
// trail and the kid stay resolvable).
const ActionTokenRevoke Action = "token_revoke"

// TokenRevokePayload accompanies an ActionTokenRevoke audit row. Reason is
// the optional free-text from the admin request. BySuperuser / Actor mirror
// TokenMintPayload.
type TokenRevokePayload struct {
	Kid         string `json:"kid"`
	Reason      string `json:"reason,omitempty"`
	BySuperuser bool   `json:"by_superuser,omitempty"`
	Actor       string `json:"actor,omitempty"`
}
