package adminusers

import (
	"net/http"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/roles"
)

func init() {
	register(func() {
		// POST /api/admin/users/{id}/roles — admin grants a role to a user.
		// Body: {"slug":"..."}
		//
		// Responses:
		//   200 — role granted (or already held; idempotent no-op response)
		//   400 — missing id or slug; slug doesn't exist
		//   403 — caller isn't admin (handled by middleware)
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

			caller := e.Auth
			if err := roles.Grant(e.App, target.Id, body.Slug, caller); err != nil {
				e.App.Logger().Error("adminusers: roles.Grant failed", "user", target.Id, "slug", body.Slug, "err", err)
				return apis.NewInternalServerError("grant failed", err)
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
		//   403 — caller isn't admin (handled by middleware)
		//   404 — target user doesn't exist
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

			caller := e.Auth
			if err := roles.Revoke(e.App, target.Id, slug, caller, body.Reason); err != nil {
				e.App.Logger().Error("adminusers: roles.Revoke failed", "user", target.Id, "slug", slug, "err", err)
				return apis.NewInternalServerError("revoke failed", err)
			}

			return e.JSON(http.StatusOK, map[string]any{
				"user_id": target.Id,
				"slug":    slug,
				"revoked": true,
			})
		})
	})
}
