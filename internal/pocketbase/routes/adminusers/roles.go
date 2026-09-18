package adminusers

import (
	"errors"
	"net/http"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"

	"github.com/xemu-cartographer/xemu-cartographer/internal/authz"
	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/pb"
)

func init() {
	register(func() {
		// POST /api/admin/users/{id}/roles — admin grants a role to a user.
		// Body: {"slug":"..."}
		//
		// Responses:
		//   200 — role granted (or already held; idempotent no-op response)
		//   400 — missing id or slug; slug doesn't exist; slug is
		//         "anonymous" (the console door row, never a user role)
		//   403 — caller isn't admin (middleware), or the role's level
		//         exceeds the caller's (tiered grants, A.7)
		//   404 — target user doesn't exist
		Group.POST("/{id}/roles", func(e *core.RequestEvent) error {
			userID := e.Request.PathValue("id")
			if userID == "" {
				return apis.NewBadRequestError("user id is required", nil)
			}

			var body struct {
				Slug string `json:"slug"`
			}
			if err := e.BindBody(&body); err != nil {
				return apis.NewBadRequestError("invalid body", err)
			}
			if body.Slug == "" {
				return apis.NewBadRequestError("slug is required", nil)
			}

			target, err := e.App.FindRecordById("users", userID)
			if err != nil {
				return apis.NewNotFoundError("user not found", err)
			}

			if _, err := e.App.FindFirstRecordByData("roles", "slug", body.Slug); err != nil {
				return apis.NewBadRequestError("role slug not found: "+body.Slug, err)
			}

			if err := pb.Grant(e.App, pb.Default(), pb.Get(e), target.Id, body.Slug, ""); err != nil {
				return roleMutationError(e, "grant", target.Id, body.Slug, err)
			}

			return e.JSON(http.StatusOK, map[string]any{
				"user_id": target.Id,
				"slug":    body.Slug,
				"granted": true,
			})
		})

		// DELETE /api/admin/users/{id}/roles/{slug} — admin revokes a role.
		// Optional body: {"reason":"..."}
		//
		// Responses:
		//   200 — role revoked (or wasn't held; idempotent)
		//   400 — missing id or slug
		//   403 — caller isn't admin (middleware), or the role's level
		//         exceeds the caller's (tiered revokes, A.7)
		//   404 — target user doesn't exist
		//   409 — the target is the last admin (§4.4)
		Group.DELETE("/{id}/roles/{slug}", func(e *core.RequestEvent) error {
			userID := e.Request.PathValue("id")
			slug := e.Request.PathValue("slug")
			if userID == "" || slug == "" {
				return apis.NewBadRequestError("id and slug are required", nil)
			}

			target, err := e.App.FindRecordById("users", userID)
			if err != nil {
				return apis.NewNotFoundError("user not found", err)
			}

			var body struct {
				Reason string `json:"reason"`
			}
			_ = e.BindBody(&body) // body is optional

			if err := pb.Revoke(e.App, pb.Default(), pb.Get(e), target.Id, slug, body.Reason); err != nil {
				return roleMutationError(e, "revoke", target.Id, slug, err)
			}

			return e.JSON(http.StatusOK, map[string]any{
				"user_id": target.Id,
				"slug":    slug,
				"revoked": true,
			})
		})
	})
}

// roleMutationError maps a pb.Grant / pb.Revoke failure to the route's
// status (design §7.1 R-4): ErrUnknownRole → 400 (the same answer as the
// slug pre-check), ErrRoleNotGrantable → 400 (the "anonymous" door row is
// not a user role), ErrLevelTooLow → 403, ErrLastAdmin → 409, and any other
// authz denial → 403; everything else is the historical 500.
func roleMutationError(e *core.RequestEvent, verb, userID, slug string, err error) error {
	switch {
	case errors.Is(err, authz.ErrUnknownRole):
		return apis.NewBadRequestError("role slug not found: "+slug, err)
	case errors.Is(err, pb.ErrRoleNotGrantable):
		return apis.NewBadRequestError("role cannot be granted to a user: "+slug, err)
	case errors.Is(err, authz.ErrLevelTooLow):
		return apis.NewForbiddenError("role level exceeds yours", err)
	case errors.Is(err, authz.ErrLastAdmin):
		return apis.NewApiError(http.StatusConflict, "cannot revoke the last admin", err)
	case errors.Is(err, pb.ErrForbidden):
		return apis.NewForbiddenError("forbidden", err)
	}
	e.App.Logger().Error("adminusers: pb."+verb+" failed", "user", userID, "slug", slug, "err", err)
	return apis.NewInternalServerError(verb+" failed", err)
}
