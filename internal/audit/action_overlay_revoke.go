package audit

// ActionOverlayRevoke records revoking an overlay token (M10). After this the
// WS handshake rejects the token even though its signature is still valid until
// expiry — revocation is the real safety net for long-lived overlay tokens.
const ActionOverlayRevoke Action = "overlay_revoke"

// OverlayRevokePayload accompanies an ActionOverlayRevoke audit row.
type OverlayRevokePayload struct {
	Room string `json:"room"`
	Kid  string `json:"kid"`
}
