package audit

// ActionOverlayMint records minting a scoped, read-only overlay token (M10).
// The target is the overlay_tokens row; the payload carries the room it's
// bound to + the kid (for cross-referencing revocations) + expiry.
const ActionOverlayMint Action = "overlay_mint"

// OverlayMintPayload accompanies an ActionOverlayMint audit row.
type OverlayMintPayload struct {
	Room      string `json:"room"`
	Kid       string `json:"kid"`
	ExpiresAt string `json:"expires_at,omitempty"`
	Label     string `json:"label,omitempty"`
}
