package overlaytoken

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/audit"
)

// DefaultTTL is the default overlay-token lifetime. Long-lived on purpose
// (set-and-forget for OBS; revocation is the real safety net). Tunable per-mint
// (the request's ttl_seconds) or globally via OVERLAY_TOKEN_TTL_HOURS (read in
// main.go and passed to Mint).
const DefaultTTL = 90 * 24 * time.Hour // 90 days

// Minted is the result of Mint.
type Minted struct {
	Token     string
	Kid       string
	Room      string
	ExpiresAt time.Time
}

// Mint signs a new read-only token bound to room, records it in the
// overlay_tokens registry (making it revocable), and writes an audit row.
// mintedBy may be nil (system). `now` is injected for testability.
func Mint(app core.App, room, label string, ttl time.Duration, mintedBy *core.Record, now time.Time) (Minted, error) {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	kid, err := newKid()
	if err != nil {
		return Minted{}, err
	}
	exp := now.Add(ttl)
	token, err := Default.Sign(room, kid, ttl, now)
	if err != nil {
		return Minted{}, err
	}

	col, err := app.FindCollectionByNameOrId("overlay_tokens")
	if err != nil {
		return Minted{}, err
	}
	rec := core.NewRecord(col)
	rec.Set("kid", kid)
	rec.Set("room", room)
	rec.Set("scope", ScopeRead)
	rec.Set("label", label)
	rec.Set("expires_at", exp)
	rec.Set("revoked", false)
	if mintedBy != nil {
		rec.Set("minted_by", mintedBy.Id)
	}
	if err := app.Save(rec); err != nil {
		return Minted{}, err
	}

	_ = audit.Write(app, mintedBy, audit.ActionOverlayMint, rec, audit.OverlayMintPayload{
		Room:      room,
		Kid:       kid,
		ExpiresAt: exp.Format(time.RFC3339),
		Label:     label,
	})

	return Minted{Token: token, Kid: kid, Room: room, ExpiresAt: exp}, nil
}

// Active reports whether kid is a live token for room: it exists, isn't
// revoked, isn't past expiry, and its registry room matches. Used by the WS
// handshake after the signature/expiry crypto check.
func Active(app core.App, kid, room string, now time.Time) bool {
	rec, err := app.FindFirstRecordByFilter("overlay_tokens", "kid = {:k}", dbx.Params{"k": kid})
	if err != nil || rec == nil {
		return false
	}
	if rec.GetBool("revoked") {
		return false
	}
	if rec.GetString("room") != room {
		return false
	}
	if exp := rec.GetDateTime("expires_at").Time(); !exp.IsZero() && now.After(exp) {
		return false
	}
	return true
}

// VerifyActive is the WS-handshake one-shot: verify the token's signature +
// expiry (crypto) AND confirm it's live in the registry (not revoked). Returns
// the bound room on success. Fails closed.
func VerifyActive(app core.App, tokenStr string, now time.Time) (room string, ok bool) {
	claims, err := Default.Verify(tokenStr)
	if err != nil {
		return "", false
	}
	if !Active(app, claims.Kid, claims.Room, now) {
		return "", false
	}
	return claims.Room, true
}

// Revoke marks the token revoked + writes an audit row. No-op (nil) when the
// kid is missing or already revoked. by may be nil.
func Revoke(app core.App, kid string, by *core.Record) error {
	rec, err := app.FindFirstRecordByFilter("overlay_tokens", "kid = {:k}", dbx.Params{"k": kid})
	if err != nil || rec == nil {
		return nil
	}
	if rec.GetBool("revoked") {
		return nil
	}
	rec.Set("revoked", true)
	rec.Set("revoked_at", time.Now())
	if by != nil {
		rec.Set("revoked_by", by.Id)
	}
	if err := app.Save(rec); err != nil {
		return err
	}
	_ = audit.Write(app, by, audit.ActionOverlayRevoke, rec, audit.OverlayRevokePayload{
		Room: rec.GetString("room"),
		Kid:  kid,
	})
	return nil
}

func newKid() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
