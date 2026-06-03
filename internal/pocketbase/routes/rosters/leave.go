package rosters

import (
	"net/http"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"

	"github.com/Stewball32/xemu-cartographer/internal/teamlog"
)

func init() {
	register(func() {
		// POST /api/rosters/{id}/leave — caller stamps left_at on their own
		// active roster row. No notification fan-out (self-action); a
		// team_log EventMemberLeft row marks the timeline.
		//
		// Guards:
		//   400 — roster row already has left_at; row is malformed
		//   403 — caller doesn't own the gamertag on the row
		//   404 — roster row doesn't exist
		Group.POST("/{id}/leave", func(e *core.RequestEvent) error {
			caller := e.Auth
			if caller == nil {
				return apis.NewUnauthorizedError("authentication required", nil)
			}
			id := e.Request.PathValue("id")
			if id == "" {
				return apis.NewBadRequestError("id is required", nil)
			}

			row, err := e.App.FindRecordById("rosters", id)
			if err != nil {
				return apis.NewNotFoundError("roster row not found", err)
			}
			if !row.GetDateTime("left_at").IsZero() {
				return apis.NewBadRequestError("roster row already left", nil)
			}

			gamertagID := row.GetString("gamertag")
			if gamertagID == "" {
				return apis.NewBadRequestError("roster row has no gamertag", nil)
			}
			gt, err := e.App.FindRecordById("gamertags", gamertagID)
			if err != nil {
				return apis.NewInternalServerError("gamertag lookup failed", err)
			}
			if gt.GetString("user") != caller.Id {
				return apis.NewForbiddenError("you may only leave your own roster row", nil)
			}

			team, err := e.App.FindRecordById("teams", row.GetString("team"))
			if err != nil {
				return apis.NewInternalServerError("team lookup failed", err)
			}

			if leftAt, parseErr := types.ParseDateTime(time.Now().UTC()); parseErr == nil {
				row.Set("left_at", leftAt)
			}
			if err := e.App.Save(row); err != nil {
				return apis.NewInternalServerError("failed to stamp left_at", err)
			}

			if err := teamlog.Write(e.App, team, caller, teamlog.EventMemberLeft, caller, gt, teamlog.MemberLeftPayload{
				Gamertag: gt.GetString("tag"),
			}); err != nil {
				e.App.Logger().Error("leave: team_log EventMemberLeft failed", "roster", row.Id, "err", err)
			}

			return e.JSON(http.StatusOK, row)
		})
	})
}
