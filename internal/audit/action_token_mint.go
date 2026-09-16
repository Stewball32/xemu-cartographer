package audit

// ActionTokenMint records an api_tokens row being minted (machine /
// spectator / device key). The target is the api_tokens row; the secret
// never appears anywhere — only the kid, kind, label and scopes.
const ActionTokenMint Action = "token_mint"

// TokenMintPayload accompanies an ActionTokenMint audit row. ExpiresAt is
// RFC3339 or empty for a key that never expires. BySuperuser marks a PB
// superuser actor (PD-14); Actor carries the label of an actor the
// audit_log actor relation cannot reference ("discord:<snowflake>").
type TokenMintPayload struct {
	Kid         string   `json:"kid"`
	Kind        string   `json:"kind"`
	Label       string   `json:"label,omitempty"`
	Scopes      []string `json:"scopes"`
	ExpiresAt   string   `json:"expires_at,omitempty"`
	BySuperuser bool     `json:"by_superuser,omitempty"`
	Actor       string   `json:"actor,omitempty"`
}
