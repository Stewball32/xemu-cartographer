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
		// POST /api/team-membership-requests/{id}/cancel — no body.
		//
		// Initiator-only — distinct from decline (recipient). Both close
		// the request, but the notification points the other direction:
		// cancel tells the recipient the offer was withdrawn.
		//
		// Status transitions: 404 / 403 / 409 if any guard fails.
		// On success: 200 with the closed request row.
		Group.POST("/{id}/cancel", func(e *core.RequestEvent) error {
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
			if req.GetString("initiated_by") != caller.Id {
				return apis.NewForbiddenError("only the initiator may cancel this request", nil)
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

			req.Set("status", "cancelled")
			req.Set("responded_by", caller.Id)
			if err := e.App.Save(req); err != nil {
				return apis.NewInternalServerError("failed to close request", err)
			}

			// Notify the other party. For an invite the recipient is
			// subjectUser (notified that the inviter pulled back); for a
			// join request the recipients are the team's owners/managers
			// (notified that the user withdrew). Self-cancel covers both
			// invited (initiator = caller, subject = invitee) and
			// requested (initiator = caller = subject).
			callerHandleID := caller.GetString("default_gamertag")
			callerHandle := lookupHandle(e.App, callerHandleID)
			callerName := caller.GetString("name")

			if direction == "invited" {
				if subjectUser.Id != caller.Id {
					_ = notifications.Notify(e.App, subjectUser, notifications.NotifTypeTeamInviteCancelled, notifications.TeamInviteCancelledPayload{
						RequestID:       req.Id,
						TeamID:          team.Id,
						TeamName:        team.GetString("name"),
						TeamSlug:        team.GetString("slug"),
						CancelledByID:   caller.Id,
						CancelledByName: callerName,
						CancelledHandle: callerHandle,
					})
				}
				if err := teamlog.Write(e.App, team, caller, teamlog.EventInviteCancelled, subjectUser, nil, teamlog.InviteCancelledPayload{
					RequestID: req.Id,
				}); err != nil {
					e.App.Logger().Error("cancel: team_log EventInviteCancelled failed", "request", req.Id, "err", err)
				}
			} else {
				// Fan out one notification per active owner/manager so they
				// see the request close. teamperms.OwnersAndManagers
				// returns the dedup'd user list.
				payload := notifications.TeamJoinRequestCancelledPayload{
					RequestID:       req.Id,
					TeamID:          team.Id,
					TeamName:        team.GetString("name"),
					TeamSlug:        team.GetString("slug"),
					CancelledByID:   caller.Id,
					CancelledByName: callerName,
					CancelledHandle: callerHandle,
				}
				owners, ownersErr := teamperms.OwnersAndManagers(e.App, team.Id)
				if ownersErr == nil {
					for _, owner := range owners {
						if owner.Id == caller.Id {
							continue
						}
						_ = notifications.Notify(e.App, owner, notifications.NotifTypeTeamJoinRequestCancelled, payload)
					}
				}
				if err := teamlog.Write(e.App, team, caller, teamlog.EventRequestCancelled, subjectUser, nil, teamlog.RequestCancelledPayload{
					RequestID: req.Id,
				}); err != nil {
					e.App.Logger().Error("cancel: team_log EventRequestCancelled failed", "request", req.Id, "err", err)
				}
			}

			return e.JSON(http.StatusOK, req)
		})
	})
}
