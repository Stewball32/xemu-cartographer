package team_membership_requests

import (
	"net/http"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/notifications"
	"github.com/Stewball32/xemu-cartographer/internal/teamlog"
	"github.com/Stewball32/xemu-cartographer/internal/teamperms"
)

func init() {
	register(func() {
		// POST /api/team-membership-requests/{id}/decline — no body.
		//
		// For direction=invited: the invitee declines. Caller must be the
		//   request's `user`.
		// For direction=requested: an owner/manager rejects. Caller must
		//   own/manage the team.
		//
		// Status transitions: 404 / 400 / 409 if any guard fails.
		// On success: 200 with the closed request row. Notifies the other
		// party + writes EventInviteDeclined / EventRequestDeclined.
		Group.POST("/{id}/decline", func(e *core.RequestEvent) error {
			caller := e.Auth
			if caller == nil {
				return apis.NewUnauthorizedError("authentication required", nil)
			}
			reqID := e.Request.PathValue("id")
			if reqID == "" {
				return apis.NewBadRequestError("id is required", nil)
			}

			req, err := e.App.FindRecordById("team_membership_requests", reqID)
			if err != nil {
				return apis.NewNotFoundError("request not found", err)
			}
			if req.GetString("status") != "pending" {
				return apis.NewApiError(http.StatusConflict, "request is no longer pending", nil)
			}

			team, err := e.App.FindRecordById("teams", req.GetString("team"))
			if err != nil {
				return apis.NewInternalServerError("team lookup failed", err)
			}
			subjectUser, err := e.App.FindRecordById("users", req.GetString("user"))
			if err != nil {
				return apis.NewInternalServerError("subject user lookup failed", err)
			}

			direction := req.GetString("direction")
			switch direction {
			case "invited":
				if caller.Id != subjectUser.Id {
					return apis.NewForbiddenError("only the invitee may decline this invite", nil)
				}
			case "requested":
				ok, err := teamperms.IsOwnerOrManager(e.App, caller.Id, team.Id)
				if err != nil {
					return apis.NewInternalServerError("permission check failed", err)
				}
				if !ok {
					return apis.NewForbiddenError("only an owner or manager may decline this request", nil)
				}
			default:
				return apis.NewInternalServerError("unknown request direction", nil)
			}

			req.Set("status", "declined")
			req.Set("responded_by", caller.Id)
			if err := e.App.Save(req); err != nil {
				return apis.NewInternalServerError("failed to close request", err)
			}

			// Notify the originator.
			otherPartyID := req.GetString("initiated_by")
			if otherPartyID != "" && otherPartyID != caller.Id {
				other, lookupErr := e.App.FindRecordById("users", otherPartyID)
				if lookupErr == nil {
					handleID := caller.GetString("default_gamertag")
					handle := lookupHandle(e.App, handleID)
					name := caller.GetString("name")
					if direction == "invited" {
						_ = notifications.Notify(e.App, other, notifications.NotifTypeTeamInviteDeclined, notifications.TeamInviteDeclinedPayload{
							RequestID:      req.Id,
							TeamID:         team.Id,
							TeamName:       team.GetString("name"),
							TeamSlug:       team.GetString("slug"),
							DeclinerID:     caller.Id,
							DeclinerName:   name,
							DeclinerHandle: handle,
						})
					} else {
						_ = notifications.Notify(e.App, other, notifications.NotifTypeTeamJoinRequestDeclined, notifications.TeamJoinRequestDeclinedPayload{
							RequestID:      req.Id,
							TeamID:         team.Id,
							TeamName:       team.GetString("name"),
							TeamSlug:       team.GetString("slug"),
							DeclinerID:     caller.Id,
							DeclinerName:   name,
							DeclinerHandle: handle,
						})
					}
				}
			}

			if direction == "invited" {
				if err := teamlog.Write(e.App, team, caller, teamlog.EventInviteDeclined, subjectUser, nil, teamlog.InviteDeclinedPayload{
					RequestID: req.Id,
				}); err != nil {
					e.App.Logger().Error("decline: team_log EventInviteDeclined failed", "request", req.Id, "err", err)
				}
			} else {
				if err := teamlog.Write(e.App, team, caller, teamlog.EventRequestDeclined, subjectUser, nil, teamlog.RequestDeclinedPayload{
					RequestID: req.Id,
				}); err != nil {
					e.App.Logger().Error("decline: team_log EventRequestDeclined failed", "request", req.Id, "err", err)
				}
			}

			return e.JSON(http.StatusOK, req)
		})
	})
}
