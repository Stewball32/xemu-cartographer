// Package overlaytoken implements the M10 spectator/overlay auth layer: a
// scoped, revocable, read-only token for OBS browser-source overlays that
// can't carry a logged-in user's session. A token is a compact HS256 JWT
// signed with a server secret carrying {room, scope:"read", kid, exp}. The
// WS handshake accepts EITHER a logged-in user JWT (operator path, governed by
// the M09 roster gate) OR a valid overlay token (OBS path) — overlay tokens are
// strictly read-only and bound to one room.
//
// token.go is the pure crypto (sign/verify), unit-testable without a DB.
// registry.go adds the PocketBase-backed mint/revoke/active registry that makes
// tokens revocable + audited.
package overlaytoken

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ScopeRead is the only scope overlay tokens carry — they never authorize
// commands/control, only read-only room subscription.
const ScopeRead = "read"

var (
	// ErrNoSecret means the signer hasn't been Configure()d with a secret.
	ErrNoSecret = errors.New("overlaytoken: signer not configured")
	// ErrWrongScope means the token's scope isn't "read".
	ErrWrongScope = errors.New("overlaytoken: not a read-scope token")
	// ErrBadAlg means the token wasn't signed with the expected HMAC method.
	ErrBadAlg = errors.New("overlaytoken: unexpected signing method")
)

// Claims is the overlay-token payload. Room binds the token to one
// host:<name> room; Kid is the registry key used for revocation.
type Claims struct {
	Room  string `json:"room"`
	Scope string `json:"scope"`
	Kid   string `json:"kid"`
	jwt.RegisteredClaims
}

// Signer signs + verifies overlay tokens with a shared secret.
type Signer struct{ secret []byte }

// New returns a Signer with the given secret (tests construct these directly).
func New(secret []byte) *Signer { return &Signer{secret: secret} }

// Default is the process-wide signer used by the mint routes + WS handshake.
// main.go calls Configure with the server secret at startup; until then Sign /
// Verify fail closed with ErrNoSecret.
var Default = &Signer{}

// Configure sets the Default signer's secret (called once from main.go).
func Configure(secret []byte) { Default.secret = secret }

// Sign issues a read-scope token for room with the given kid, expiring ttl
// after now. now is injected so tests can mint already-expired tokens.
func (s *Signer) Sign(room, kid string, ttl time.Duration, now time.Time) (string, error) {
	if len(s.secret) == 0 {
		return "", ErrNoSecret
	}
	claims := Claims{
		Room:  room,
		Scope: ScopeRead,
		Kid:   kid,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

// Verify checks the signature + expiry (via golang-jwt) and that the scope is
// "read". Returns the parsed claims or an error (expired, bad signature, wrong
// alg/scope). Expiry is validated against the real clock.
func (s *Signer) Verify(tokenStr string) (*Claims, error) {
	if len(s.secret) == 0 {
		return nil, ErrNoSecret
	}
	var claims Claims
	_, err := jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrBadAlg
		}
		return s.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, err
	}
	if claims.Scope != ScopeRead {
		return nil, ErrWrongScope
	}
	return &claims, nil
}

// Sign / Verify on the Default signer, for the route + handshake call sites.
func Sign(room, kid string, ttl time.Duration, now time.Time) (string, error) {
	return Default.Sign(room, kid, ttl, now)
}

func Verify(tokenStr string) (*Claims, error) { return Default.Verify(tokenStr) }
