package rosters

import (
	"net/http"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"

	"github.com/Stewball32/xemu-cartographer/internal/notifications"
	"github.com/Stewball32/xemu-cartographer/internal/teamlog"
	"github.com/Stewball32/xemu-cartographer/internal/teamperms"
)

func init() {
	register(func() {
		// POST /api/rosters/{id}/remove — owner/manager stamps left_at on
		// another member's active roster row. Notifies the removed user;
		// emits EventMemberRemoved on the team timeline.
		//
		// Guards:
		//   400 — roster row already has left_at; row is malformed
		//   403 — caller is not an owner/manager of the team; OR caller
		//         owns the gamertag on the row (use /leave instead)
		//   404 — roster row doesn't exist
		Group.POST("/{id}/remove", func(e *core.RequestEvent) error {
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
			if gt.GetString("user") == caller.Id {
				return apis.NewForbiddenError("use /leave for your own roster row", nil)
			}

			team, err := e.App.FindRecordById("teams", row.GetString("team"))
			if err != nil {
				return apis.NewInternalServerError("team lookup failed", err)
			}

			ok, err := teamperms.IsOwnerOrManager(e.App, caller.Id, team.Id)
			if err != nil {
				return apis.NewInternalServerError("permission check failed", err)
			}
			if !ok {
				return apis.NewForbiddenError("only team owners or managers may remove members", nil)
			}

			// Stamp left_at + persist.
			if leftAt, parseErr := types.ParseDateTime(time.Now().UTC()); parseErr == nil {
				row.Set("left_at", leftAt)
			}
			if err := e.App.Save(row); err != nil {
				return apis.NewInternalServerError("failed to stamp left_at", err)
			}

			removedUserID := gt.GetString("user")
			removed, lookupErr := e.App.FindRecordById("users", removedUserID)
			if lookupErr == nil && removed != nil && removed.Id != caller.Id {
				removerHandleID := caller.GetString("default_gamertag")
				removerHandle := lookupHandle(e.App, removerHandleID)
				_ = notifications.Notify(e.App, removed, notifications.NotifTypeTeamRemoved, notifications.TeamRemovedPayload{
					RosterID:        row.Id,
					TeamID:          team.Id,
					TeamName:        team.GetString("name"),
					TeamSlug:        team.GetString("slug"),
					RemovedGamertag: gt.GetString("tag"),
					RemovedByID:     caller.Id,
					RemovedByName:   caller.GetString("name"),
					RemovedByHandle: removerHandle,
				})
			}

			if err := teamlog.Write(e.App, team, caller, teamlog.EventMemberRemoved, removed, gt, teamlog.MemberRemovedPayload{
				Gamertag: gt.GetString("tag"),
			}); err != nil {
				e.App.Logger().Error("remove: team_log EventMemberRemoved failed", "roster", row.Id, "err", err)
			}

			return e.JSON(http.StatusOK, row)
		})
	})
}

// lookupHandle resolves a gamertag id to its `tag`. Duplicated from
// routes/teams/invite.go — the two packages can't import each other
// without a cycle.
func lookupHandle(app core.App, gamertagID string) string {
	if gamertagID == "" {
		return ""
	}
	rec, err := app.FindRecordById("gamertags", gamertagID)
	if err != nil {
		return ""
	}
	return rec.GetString("tag")
}
