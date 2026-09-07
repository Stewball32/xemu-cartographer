package pb

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/audit"
	"github.com/Stewball32/xemu-cartographer/internal/authz"
)

// SpectatorTTL is the default expiry of a spectator key when the mint
// request names none (PD-4: OBS sources are long-lived).
const SpectatorTTL = 90 * 24 * time.Hour

// ErrLegacyToken is returned by RevokeToken for the legacy-env kid: the row
// lives in the environment, not in api_tokens (409 "unset LAN_SAVES_TOKEN
// instead").
var ErrLegacyToken = errors.New("authz: legacy-env key is managed by LAN_SAVES_TOKEN — unset the env var instead")

// ErrMintRequest flags a mint request that is well-formed scope-wise but
// references something that does not exist or an expiry in the past (400).
var ErrMintRequest = errors.New("authz: invalid mint request")

// ErrMintCeiling is returned by Mint when the caller asks for a scope wider
// than its own (403): a key can delegate at most what its minter holds.
// Superusers, internal callers and admins are exempt (PD-13: wildcard
// machine keys are admin-minted), and overlay.mint covers the overlay
// spectator grant (checkMintCeiling); the offending scope is in the message.
var ErrMintCeiling = errors.New("authz: scope exceeds the caller's own")

// MintRequest describes a new api_tokens row. Scopes are canonicalised
// before ValidateForMint; Gamertags accepts gamertags record ids or tag
// strings (PD-8); a nil ExpiresAt means never (spectator: SpectatorTTL).
type MintRequest struct {
	Kind, Label, UserID, Container, StationID string
	Scopes                                    []string
	Gamertags                                 []string
	ExpiresAt                                 *time.Time
}

// MintResult is the one-time answer: Token = "<kid>.<secret>" is shown once
// and never stored; Row is the Deps view of the new record without KeyHash.
type MintResult struct {
	Kid, Token string
	Row        authz.TokenRow
}

// TokenDetail is the ListTokenDetails projection: the Deps row (KeyHash
// always empty) plus the bookkeeping columns the admin listing shows.
type TokenDetail struct {
	authz.TokenRow
	ID         string
	MintedBy   string
	RevokedAt  time.Time
	LastUsedAt time.Time
	Created    time.Time
}

// Mint validates (ValidateForMint), authorises (Can token.mint with
// Extra["kind"], then the delegation ceiling — ErrMintCeiling for a scope
// the caller does not hold itself), stores the api_tokens row and writes
// the ActionTokenMint audit row. Device keys with no Container are bound to
// the one instance their scopes name.
func Mint(app core.App, d *PBDeps, actor authz.Principal, req MintRequest) (MintResult, error) {
	if app == nil {
		return MintResult{}, fmt.Errorf("pb.Mint: app is required")
	}
	d = depsOr(d, app)
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	scopes := authz.CanonScopes(req.Scopes)
	if err := authz.ValidateForMint(kind, scopes); err != nil {
		return MintResult{}, err
	}

	res := authz.Global()
	res.Extra = map[string]string{"kind": kind}
	if dec := authz.CanWith(d, actor, authz.ActionTokenMint, res); !dec.Allow {
		return MintResult{}, decisionError(dec)
	}
	if err := checkMintCeiling(actor, kind, scopes); err != nil {
		return MintResult{}, err
	}

	now := time.Now()
	var expires time.Time
	if req.ExpiresAt != nil {
		expires = req.ExpiresAt.UTC()
		if !expires.After(now) {
			return MintResult{}, fmt.Errorf("%w: expires_at must be in the future", ErrMintRequest)
		}
	} else if kind == "spectator" {
		expires = now.Add(SpectatorTTL).UTC()
	}

	userID := strings.TrimSpace(req.UserID)
	if userID != "" {
		if _, err := app.FindRecordById("users", userID); err != nil {
			return MintResult{}, fmt.Errorf("%w: unknown user %q", ErrMintRequest, userID)
		}
	}
	tagIDs, tags, err := resolveGamertags(app, req.Gamertags)
	if err != nil {
		return MintResult{}, err
	}
	container := strings.TrimSpace(req.Container)
	if kind == "device" && container == "" {
		container = scopesInstance(scopes)
	}

	kid, err := authz.NewKid(kind)
	if err != nil {
		return MintResult{}, err
	}
	secret, err := authz.NewSecret()
	if err != nil {
		return MintResult{}, err
	}

	col, err := app.FindCollectionByNameOrId("api_tokens")
	if err != nil {
		return MintResult{}, fmt.Errorf("pb.Mint: lookup api_tokens collection: %w", err)
	}
	rec := core.NewRecord(col)
	rec.Set("kid", kid)
	rec.Set("key_hash", authz.HashSecret(secret))
	rec.Set("kind", kind)
	rec.Set("scopes", scopes)
	rec.Set("label", strings.TrimSpace(req.Label))
	rec.Set("user", userID)
	rec.Set("container", container)
	rec.Set("station_id", strings.TrimSpace(req.StationID))
	rec.Set("gamertags", tagIDs)
	if actor.Collection == "users" && actor.UserID != "" {
		rec.Set("minted_by", actor.UserID)
	}
	if !expires.IsZero() {
		rec.Set("expires_at", expires)
	}
	if err := app.Save(rec); err != nil {
		return MintResult{}, fmt.Errorf("pb.Mint: save api_tokens: %w", err)
	}

	row := authz.TokenRow{
		Kid:       kid,
		Kind:      kind,
		Label:     rec.GetString("label"),
		UserID:    userID,
		Container: container,
		StationID: rec.GetString("station_id"),
		Scopes:    scopes,
		Gamertags: tags,
		ExpiresAt: expires,
	}
	payload := audit.TokenMintPayload{
		Kid:         kid,
		Kind:        kind,
		Label:       row.Label,
		Scopes:      scopes,
		BySuperuser: actor.Kind == authz.KindSuperuser,
		Actor:       auditActorLabel(actor),
	}
	if !expires.IsZero() {
		payload.ExpiresAt = expires.Format(time.RFC3339)
	}
	if err := audit.WriteRef(app, actorRecord(app, actor), audit.ActionTokenMint, "api_tokens", rec.Id, payload); err != nil {
		return MintResult{}, fmt.Errorf("pb.Mint: audit write: %w", err)
	}
	return MintResult{Kid: kid, Token: kid + "." + secret, Row: row}, nil
}

// RevokeToken flags the api_tokens row revoked (revoked_at now, revoked_by
// for users actors) and writes the ActionTokenRevoke audit row. Can runs
// first so an unauthorised caller learns nothing about the kid; then the
// legacy-env kid is ErrLegacyToken and an unknown kid ErrUnknownKid. A row
// already revoked is a no-op.
func RevokeToken(app core.App, d *PBDeps, actor authz.Principal, kid, reason string) error {
	if app == nil {
		return fmt.Errorf("pb.RevokeToken: app is required")
	}
	d = depsOr(d, app)
	kid = strings.TrimSpace(kid)
	if kid == "" {
		return authz.ErrUnknownKid
	}
	if dec := authz.CanWith(d, actor, authz.ActionTokenRevoke, authz.Token(kid)); !dec.Allow {
		return decisionError(dec)
	}
	if kid == authz.LegacyKid {
		return ErrLegacyToken
	}
	rec, err := findTokenRecord(app, kid)
	if err != nil {
		return authz.ErrUnknownKid
	}
	if rec.GetBool("revoked") {
		return nil
	}
	rec.Set("revoked", true)
	rec.Set("revoked_at", time.Now().UTC())
	if actor.Collection == "users" && actor.UserID != "" {
		rec.Set("revoked_by", actor.UserID)
	}
	if err := app.Save(rec); err != nil {
		return fmt.Errorf("pb.RevokeToken: save api_tokens %s: %w", kid, err)
	}
	payload := audit.TokenRevokePayload{
		Kid:         kid,
		Reason:      strings.TrimSpace(reason),
		BySuperuser: actor.Kind == authz.KindSuperuser,
		Actor:       auditActorLabel(actor),
	}
	if err := audit.WriteRef(app, actorRecord(app, actor), audit.ActionTokenRevoke, "api_tokens", rec.Id, payload); err != nil {
		return fmt.Errorf("pb.RevokeToken: audit write: %w", err)
	}
	return nil
}

// ListTokens returns the api_tokens rows (plus the legacy-env row when it
// is imported) of the given kind ("" = every kind) after Can(token.list).
// KeyHash is never populated.
func ListTokens(app core.App, d *PBDeps, actor authz.Principal, kind string) ([]authz.TokenRow, error) {
	details, err := ListTokenDetails(app, d, actor, kind)
	if err != nil {
		return nil, err
	}
	out := make([]authz.TokenRow, 0, len(details))
	for _, det := range details {
		out = append(out, det.TokenRow)
	}
	return out, nil
}

// ListTokenDetails is ListTokens with the bookkeeping columns the admin
// listing renders (minted_by, revoked_at, last_used_at). Newest first; the
// legacy-env row, when present and matching kind, leads. KeyHash is never
// populated.
func ListTokenDetails(app core.App, d *PBDeps, actor authz.Principal, kind string) ([]TokenDetail, error) {
	if app == nil {
		return nil, fmt.Errorf("pb.ListTokens: app is required")
	}
	d = depsOr(d, app)
	kind = strings.ToLower(strings.TrimSpace(kind))
	if dec := authz.CanWith(d, actor, authz.ActionTokenList, authz.Global()); !dec.Allow {
		return nil, decisionError(dec)
	}

	out := []TokenDetail{}
	if legacy := d.legacyRow(); legacy != nil && (kind == "" || kind == legacy.Kind) {
		legacy.KeyHash = ""
		out = append(out, TokenDetail{TokenRow: *legacy})
	}

	filter, params := "id != ''", dbx.Params{}
	if kind != "" {
		filter, params = "kind = {:kind}", dbx.Params{"kind": kind}
	}
	rows, err := app.FindRecordsByFilter("api_tokens", filter, "-created", 0, 0, params)
	if err != nil {
		return nil, fmt.Errorf("pb.ListTokens: %w", err)
	}
	for _, rec := range rows {
		row := tokenRowFromRecord(app, rec)
		row.KeyHash = ""
		det := TokenDetail{
			TokenRow: row,
			ID:       rec.Id,
			MintedBy: rec.GetString("minted_by"),
		}
		if v := rec.GetDateTime("revoked_at"); !v.IsZero() {
			det.RevokedAt = v.Time()
		}
		if v := rec.GetDateTime("last_used_at"); !v.IsZero() {
			det.LastUsedAt = v.Time()
		}
		if v := rec.GetDateTime("created"); !v.IsZero() {
			det.Created = v.Time()
		}
		out = append(out, det)
	}
	return out, nil
}

// resolveGamertags maps mint-request gamertags (record ids or tag strings)
// to record ids + sanitized tags; an unknown entry is ErrMintRequest.
func resolveGamertags(app core.App, in []string) (ids, tags []string, err error) {
	for _, g := range in {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		rec, lookupErr := app.FindRecordById("gamertags", g)
		if lookupErr != nil {
			rec, lookupErr = app.FindFirstRecordByFilter(
				"gamertags",
				"sanitized = {:tag} || tag = {:raw}",
				dbx.Params{"tag": strings.ToLower(g), "raw": g},
			)
		}
		if lookupErr != nil || rec == nil {
			return nil, nil, fmt.Errorf("%w: unknown gamertag %q", ErrMintRequest, g)
		}
		ids = append(ids, rec.Id)
	}
	return ids, sanitizedTagsByID(app, ids), nil
}

// checkMintCeiling enforces the delegation ceiling: every requested scope
// must be one the actor itself holds. Superusers and internal callers pass;
// so does an admin (the role, or a level at or above the admin seed's —
// PD-13 puts wildcard machine keys in admin hands). Everyone else, machine
// keys included, may only hand down what it has: a literal scope needs a
// scope that grants it (MatchAny, the scope as the want); a wildcard scope
// needs a held wildcard at least as wide (scopeCovers) — a literal never
// covers a wildcard, so token.mint:machine alone cannot mint "*".
//
// One grant is read by meaning rather than by pattern: overlay.mint is the
// PD-5 licence to mint overlay spectator keys, so for a spectator key its
// holder covers the overlay grant on any instance — read_state plus the
// OverlayClasses rooms, one literal instance (authz.OverlayMintCovers) —
// without carrying those scopes itself; the seeded overlay_manager holds
// neither room.join nor a per-instance read_state, and Studio mints exactly
// that list. Any other spectator scope (scraper.state, another room class)
// and every machine / device scope stays under the literal ceiling.
func checkMintCeiling(actor authz.Principal, kind string, scopes []string) error {
	switch actor.Kind {
	case authz.KindSuperuser, authz.KindInternal:
		return nil
	case authz.KindPBUser:
		if actor.IsAdmin() {
			return nil
		}
		if seed, ok := authz.SeedRole("admin"); ok && actor.Level >= seed.Level {
			return nil
		}
	}
	overlay := kind == "spectator" && holdsScope(actor.Scopes, string(authz.ActionOverlayMint))
	for _, s := range scopes {
		if overlay && authz.OverlayMintCovers(s) {
			continue
		}
		if !holdsScope(actor.Scopes, s) {
			return fmt.Errorf("%w: %q", ErrMintCeiling, s)
		}
	}
	return nil
}

// holdsScope reports whether one of held grants scope s in full.
func holdsScope(held []string, s string) bool {
	want, err := authz.ParseScope(s)
	if err != nil {
		return false
	}
	if !authz.IsWildcard(s) {
		_, ok := authz.MatchAny(held, s)
		return ok
	}
	for _, h := range held {
		pat, err := authz.ParseScope(h)
		if err != nil {
			continue
		}
		if scopeCovers(pat, want) {
			return true
		}
	}
	return false
}

// scopeCovers reports whether the held pattern is at least as wide as want:
// every want string the requested pattern would grant, the held one grants
// too. Bare "*" covers everything; "<family>.*" covers the family and its
// sub-families; an exact action covers only itself. A held selector with no
// segments covers every selector only for a family / "*" pattern; a lone
// "*" selector covers any; otherwise the segment counts must agree and each
// held segment is "*" or equal (a "*" want segment needs a "*" held one).
func scopeCovers(held, want authz.ScopePat) bool {
	switch {
	case held.Any:
		return true
	case held.Family:
		if want.Any {
			return false
		}
		inFamily := strings.HasPrefix(want.Action, held.Action+".")
		if want.Family && want.Action == held.Action {
			inFamily = true
		}
		if !inFamily {
			return false
		}
	default:
		if want.Any || want.Family || want.Action != held.Action {
			return false
		}
	}
	if len(held.Selector) == 0 {
		return held.Family || len(want.Selector) == 0
	}
	if len(held.Selector) == 1 && held.Selector[0] == "*" {
		return true
	}
	if len(held.Selector) != len(want.Selector) {
		return false
	}
	for i, seg := range held.Selector {
		if seg != "*" && seg != want.Selector[i] {
			return false
		}
	}
	return true
}

// scopesInstance returns the single instance a validated spectator / device
// scope list names ("" when it cannot be read).
func scopesInstance(scopes []string) string {
	inst := ""
	for _, s := range scopes {
		pat, err := authz.ParseScope(s)
		if err != nil || len(pat.Selector) == 0 {
			return ""
		}
		named := pat.Selector[0]
		if authz.Action(pat.Action) == authz.ActionRoomJoin {
			if len(pat.Selector) < 2 {
				return ""
			}
			named = pat.Selector[1]
		}
		if inst != "" && named != inst {
			return ""
		}
		inst = named
	}
	return inst
}
