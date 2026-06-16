package adminusers

import (
	"net/http"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

// GET /api/admin/users/role-assignments returns every user_roles row in the
// system, with the role's slug and the user's id, for the M8g /admin/roles/
// Assignments tab. The user_roles collection's PB list rule is self-only
// (admit-by-subquery doesn't work cleanly on the user_roles collection
// itself — see schema/user_roles.go), so the FE can't enumerate the join
// table through the SDK. This route bypasses rules via app.FindRecords +
// RequireAdmin middleware on the group.
//
// Shape:
//
//	[{"user": "<userId>", "role_slug": "<slug>"}, ...]
type roleAssignment struct {
	User     string `json:"user"`
	RoleSlug string `json:"role_slug"`
}

func init() {
	register(func() {
		Group.GET("/role-assignments", func(e *core.RequestEvent) error {
			rows, err := e.App.FindAllRecords("user_roles")
			if err != nil {
				return apis.NewInternalServerError("failed to load user_roles", err)
			}
			out := make([]roleAssignment, 0, len(rows))
			if len(rows) == 0 {
				return e.JSON(http.StatusOK, out)
			}
			for _, expandErr := range e.App.ExpandRecords(rows, []string{"role"}, nil) {
				if expandErr != nil {
					e.App.Logger().Error("role-assignments: role expand failed", "err", expandErr)
				}
			}
			for _, r := range rows {
				slug := ""
				if role := r.ExpandedOne("role"); role != nil {
					slug = role.GetString("slug")
				}
				out = append(out, roleAssignment{
					User:     r.GetString("user"),
					RoleSlug: slug,
				})
			}
			return e.JSON(http.StatusOK, out)
		})
	})
}
