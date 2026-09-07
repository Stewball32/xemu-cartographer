package adminusers

import (
	"net/http"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/audit"
	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb"
)

func init() {
	register(func() {
		Group.POST("/{id}/ban", handleBan)
		Group.POST("/{id}/unban", handleUnban)
		Group.POST("/{id}/timeout", handleTimeout)
	})
}

// moderationTarget is lookupTarget followed by the user.moderate check on
// the loaded user (design §2.3 / §4: the ban / unban / timeout action, the
// same one the users_ban_transitions hook enforces on the record-API path
// these handlers bypass with app.Save). The group's admin.users gate only
// proves the caller may reach /api/admin/users; a role holding admin.users
// without user.moderate is refused here with the apis 403.
func moderationTarget(e *core.RequestEvent) (*core.Record, error) {
	target, err := lookupTarget(e)
	if err != nil {
		return nil, err
	}
	if err := pb.Check(pb.Default(), e, authz.ActionUserModerate, authz.User(target.Id)); err != nil {
		return nil, err
	}
	return target, nil
}

// handleBan — POST /api/admin/users/{id}/ban — applies an indefinite ban.
// Body: {"reason":"..."} (optional)
//
// Sets is_banned=true with empty banned_until; writes ActionBan audit row
// carrying the supplied reason. Bypasses the users_ban_transitions hook
// (which fires on OnRecordUpdateRequest) by using app.Save directly, so
// the audit row isn't duplicated.
func handleBan(e *core.RequestEvent) error {
	target, err := moderationTarget(e)
	if err != nil {
		return err
	}
	if target.Id == e.Auth.Id {
		return apis.NewBadRequestError("cannot ban yourself", nil)
	}

	var body struct {
		Reason string `json:"reason"`
	}
	_ = e.BindBody(&body)

	target.Set("is_banned", true)
	target.Set("banned_until", "")
	if err := e.App.Save(target); err != nil {
		return apis.NewInternalServerError("ban failed", err)
	}

	if err := audit.Write(e.App, e.Auth, audit.ActionBan, target, audit.BanPayload{
		Reason: body.Reason,
	}); err != nil {
		e.App.Logger().Error("adminusers: audit ActionBan failed", "user", target.Id, "err", err)
	}

	return e.JSON(http.StatusOK, map[string]any{
		"user_id":   target.Id,
		"is_banned": true,
	})
}

// handleUnban — POST /api/admin/users/{id}/unban — lifts an active ban.
// Body: {"reason":"..."} (optional)
//
// Clears is_banned + banned_until; writes ActionUnban audit row. 400 if
// the user wasn't banned.
func handleUnban(e *core.RequestEvent) error {
	target, err := moderationTarget(e)
	if err != nil {
		return err
	}
	if !target.GetBool("is_banned") {
		return apis.NewBadRequestError("user is not currently banned", nil)
	}

	var body struct {
		Reason string `json:"reason"`
	}
	_ = e.BindBody(&body)

	target.Set("is_banned", false)
	target.Set("banned_until", "")
	if err := e.App.Save(target); err != nil {
		return apis.NewInternalServerError("unban failed", err)
	}

	if err := audit.Write(e.App, e.Auth, audit.ActionUnban, target, audit.UnbanPayload{
		Reason: body.Reason,
	}); err != nil {
		e.App.Logger().Error("adminusers: audit ActionUnban failed", "user", target.Id, "err", err)
	}

	return e.JSON(http.StatusOK, map[string]any{
		"user_id":   target.Id,
		"is_banned": false,
	})
}

// handleTimeout — POST /api/admin/users/{id}/timeout — time-bounded ban.
// Body: {"expires_at":"2026-06-04T10:00:00.000Z","reason":"..."}
//
// Sets is_banned=true + banned_until=expires_at; writes ActionTimeout
// audit row with the expiry. expires_at must be a non-empty string that
// PB can parse as a datetime; we don't validate the format here — the
// schema's date field will reject bad input on save.
func handleTimeout(e *core.RequestEvent) error {
	target, err := moderationTarget(e)
	if err != nil {
		return err
	}
	if target.Id == e.Auth.Id {
		return apis.NewBadRequestError("cannot time out yourself", nil)
	}

	var body struct {
		ExpiresAt string `json:"expires_at"`
		Reason    string `json:"reason"`
	}
	if err := e.BindBody(&body); err != nil {
		return apis.NewBadRequestError("invalid body", err)
	}
	if body.ExpiresAt == "" {
		return apis.NewBadRequestError("expires_at is required", nil)
	}

	target.Set("is_banned", true)
	target.Set("banned_until", body.ExpiresAt)
	if err := e.App.Save(target); err != nil {
		return apis.NewBadRequestError("timeout failed (invalid expires_at?)", err)
	}

	if err := audit.Write(e.App, e.Auth, audit.ActionTimeout, target, audit.TimeoutPayload{
		Reason:    body.Reason,
		ExpiresAt: body.ExpiresAt,
	}); err != nil {
		e.App.Logger().Error("adminusers: audit ActionTimeout failed", "user", target.Id, "err", err)
	}

	return e.JSON(http.StatusOK, map[string]any{
		"user_id":      target.Id,
		"is_banned":    true,
		"banned_until": body.ExpiresAt,
	})
}

func lookupTarget(e *core.RequestEvent) (*core.Record, error) {
	id := e.Request.PathValue("id")
	if id == "" {
		return nil, apis.NewBadRequestError("user id is required", nil)
	}
	rec, err := e.App.FindRecordById("users", id)
	if err != nil {
		return nil, apis.NewNotFoundError("user not found", err)
	}
	return rec, nil
}
