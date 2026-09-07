package tokens

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"

	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb"
)

func init() {
	register(func() {
		// POST /api/admin/tokens — mint a key.
		// Body: {kind, label, scopes[], user?, container?, station_id?, gamertags?[], expires_at?}
		//
		// Responses:
		//   201 — {kid, token, kind, label, scopes, expires_at}; token is shown once
		//   400 — {error} on ValidateForMint / body errors
		//   401 — no credential
		//   403 — caller lacks token.mint:<kind> (spectator: overlay.mint), or
		//         {error} when a requested scope exceeds the caller's own
		Group.POST("", handleMint)

		// GET /api/admin/tokens?kind= — list keys (never the hash).
		Group.GET("", handleList)

		// DELETE /api/admin/tokens/{kid} — revoke. Optional body {reason}.
		//
		// Responses:
		//   200 — {kid, revoked:true}
		//   404 — unknown kid
		//   409 — legacy-env (unset LAN_SAVES_TOKEN instead)
		Group.DELETE("/{kid}", handleRevoke)
	})
}

// mintBody is the POST payload.
type mintBody struct {
	Kind      string   `json:"kind"`
	Label     string   `json:"label"`
	Scopes    []string `json:"scopes"`
	User      string   `json:"user"`
	Container string   `json:"container"`
	StationID string   `json:"station_id"`
	Gamertags []string `json:"gamertags"`
	ExpiresAt string   `json:"expires_at"`
}

// mintAction is the handler-level check for a kind: overlay.mint for
// spectator keys (PD-5), token.mint with Extra["kind"] otherwise. pb.Mint
// applies the token.mint rule again on top.
func mintAction(kind string) (authz.Action, authz.Resource) {
	if kind == "spectator" {
		return authz.ActionOverlayMint, authz.Global()
	}
	res := authz.Global()
	res.Extra = map[string]string{"kind": kind}
	return authz.ActionTokenMint, res
}

func handleMint(e *core.RequestEvent) error {
	d := pb.Default()

	// Fail closed before the body is read: a bad credential or no credential
	// at all gets its 401 here and never reaches the body-validation 400s
	// below. Anonymous can never mint (kind gate), and REST never yields a
	// bound anonymous principal, so the kind alone decides. The per-kind
	// action check follows once the kind is known (the stored principal is
	// reused by pb.Check).
	p, err := pb.ResolveRequest(e.App, d, e)
	if err != nil {
		return apis.NewUnauthorizedError(strings.TrimPrefix(err.Error(), "authz: "), nil)
	}
	if p.Kind == authz.KindAnonymous {
		return apis.NewUnauthorizedError("authentication required", nil)
	}

	var body mintBody
	if err := e.BindBody(&body); err != nil {
		return badRequest(e, "invalid body: "+err.Error())
	}
	kind := strings.ToLower(strings.TrimSpace(body.Kind))
	if _, ok := authz.KidPrefix[kind]; !ok {
		return badRequest(e, "kind must be one of machine, spectator, device")
	}

	action, res := mintAction(kind)
	if err := pb.Check(d, e, action, res); err != nil {
		return err
	}

	req := pb.MintRequest{
		Kind:      kind,
		Label:     body.Label,
		UserID:    body.User,
		Container: body.Container,
		StationID: body.StationID,
		Scopes:    body.Scopes,
		Gamertags: body.Gamertags,
	}
	if s := strings.TrimSpace(body.ExpiresAt); s != "" {
		dt, err := types.ParseDateTime(s)
		if err != nil || dt.IsZero() {
			return badRequest(e, "expires_at: invalid datetime")
		}
		t := dt.Time()
		req.ExpiresAt = &t
	}

	result, err := pb.Mint(e.App, d, pb.Get(e), req)
	if err != nil {
		return mapMintError(e, err)
	}
	return e.JSON(http.StatusCreated, map[string]any{
		"kid":        result.Kid,
		"token":      result.Token,
		"kind":       result.Row.Kind,
		"label":      result.Row.Label,
		"scopes":     result.Row.Scopes,
		"expires_at": formatTime(result.Row.ExpiresAt),
	})
}

func handleList(e *core.RequestEvent) error {
	d := pb.Default()
	if err := pb.Check(d, e, authz.ActionTokenList, authz.Global()); err != nil {
		return err
	}
	kind := strings.ToLower(strings.TrimSpace(e.Request.URL.Query().Get("kind")))
	if kind != "" {
		if _, ok := authz.KidPrefix[kind]; !ok {
			return badRequest(e, "kind must be one of machine, spectator, device")
		}
	}
	details, err := pb.ListTokenDetails(e.App, d, pb.Get(e), kind)
	if err != nil {
		return mapListError(e, err)
	}
	out := make([]map[string]any, 0, len(details))
	for _, det := range details {
		out = append(out, map[string]any{
			"kid":          det.Kid,
			"kind":         det.Kind,
			"label":        det.Label,
			"scopes":       det.Scopes,
			"user":         det.UserID,
			"container":    det.Container,
			"station_id":   det.StationID,
			"gamertags":    det.Gamertags,
			"expires_at":   formatTime(det.ExpiresAt),
			"revoked":      det.Revoked,
			"revoked_at":   formatTime(det.RevokedAt),
			"last_used_at": formatTime(det.LastUsedAt),
			"minted_by":    det.MintedBy,
			"created":      formatTime(det.Created),
		})
	}
	return e.JSON(http.StatusOK, map[string]any{"tokens": out})
}

func handleRevoke(e *core.RequestEvent) error {
	d := pb.Default()
	kid := strings.TrimSpace(e.Request.PathValue("kid"))
	if kid == "" {
		return badRequest(e, "kid is required")
	}
	if err := pb.Check(d, e, authz.ActionTokenRevoke, authz.Token(kid)); err != nil {
		return err
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if err := e.BindBody(&body); err != nil {
		return badRequest(e, "invalid body: "+err.Error())
	}
	if err := pb.RevokeToken(e.App, d, pb.Get(e), kid, body.Reason); err != nil {
		switch {
		case errors.Is(err, pb.ErrLegacyToken):
			return e.JSON(http.StatusConflict, map[string]string{"error": "unset LAN_SAVES_TOKEN instead"})
		case errors.Is(err, authz.ErrUnknownKid):
			return e.JSON(http.StatusNotFound, map[string]string{"error": "unknown kid"})
		case errors.Is(err, pb.ErrForbidden):
			return apis.NewForbiddenError("forbidden", nil)
		}
		e.App.Logger().Error("tokens: revoke failed", "kid", kid, "err", err)
		return apis.NewInternalServerError("revoke failed", err)
	}
	return e.JSON(http.StatusOK, map[string]any{"kid": kid, "revoked": true})
}

// mapMintError maps pb.Mint errors: validation → 400 {error}, Can → 403,
// delegation ceiling → 403 {error} naming the scope, else 500.
func mapMintError(e *core.RequestEvent, err error) error {
	switch {
	case errors.Is(err, authz.ErrScopeSyntax),
		errors.Is(err, authz.ErrWildcardKind),
		errors.Is(err, authz.ErrSpectatorInstance),
		errors.Is(err, pb.ErrMintRequest):
		return badRequest(e, err.Error())
	case errors.Is(err, pb.ErrMintCeiling):
		return e.JSON(http.StatusForbidden, map[string]string{"error": strings.TrimPrefix(err.Error(), "authz: ")})
	case errors.Is(err, pb.ErrForbidden):
		return apis.NewForbiddenError("forbidden", nil)
	}
	e.App.Logger().Error("tokens: mint failed", "err", err)
	return apis.NewInternalServerError("mint failed", err)
}

func mapListError(e *core.RequestEvent, err error) error {
	if errors.Is(err, pb.ErrForbidden) {
		return apis.NewForbiddenError("forbidden", nil)
	}
	e.App.Logger().Error("tokens: list failed", "err", err)
	return apis.NewInternalServerError("list failed", err)
}

// badRequest answers 400 with the §6.3 {"error": <Go error string>} shape.
func badRequest(e *core.RequestEvent, msg string) error {
	return e.JSON(http.StatusBadRequest, map[string]string{"error": msg})
}

// formatTime renders a time as RFC3339 UTC, "" for the zero value.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
