package teams

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
		// POST /api/teams/{teamId}/invite — owner/manager extends an invite.
		// Body: {"user_id":"..."}
		//
		// Guards (in order, each returns the documented status):
		//   400 — missing teamId or user_id; team is blocked; target user is
		//         soft-deleted; target has no default_gamertag set or its
		//         default is blocked
		//   403 — caller is not an owner/manager of the team
		//   404 — team or target user doesn't exist
		//   409 — target already has an active roster row on the team, OR a
		//         pending request for (team, user) already exists in either
		//         direction
		//
		// On success: 201 with the created team_membership_requests row;
		// notification fanned out to target; team_log EventInviteCreated.
		Group.POST("/{teamId}/invite", func(e *core.RequestEvent) error {
			teamID := e.Request.PathValue("teamId")
			if teamID == "" {
				return apis.NewBadRequestError("teamId is required", nil)
			}

			var body struct {
				UserID string `json:"user_id"`
			}
			if err := e.BindBody(&body); err != nil {
				return apis.NewBadRequestError("invalid body", err)
			}
			if body.UserID == "" {
				return apis.NewBadRequestError("user_id is required", nil)
			}

			caller := e.Auth
			if caller == nil {
				return apis.NewUnauthorizedError("authentication required", nil)
			}

			team, err := e.App.FindRecordById("teams", teamID)
			if err != nil {
				return apis.NewNotFoundError("team not found", err)
			}
			if team.GetString("status") == "blocked" {
				return apis.NewBadRequestError("team is blocked; invites disabled", nil)
			}

			// Owner/manager guard. Direct PB rule expression can't help us
			// here because the route isn't a record-mutation surface — we're
			// writing into a different collection on the team's behalf.
			ok, err := teamperms.IsOwnerOrManager(e.App, caller.Id, team.Id)
			if err != nil {
				e.App.Logger().Error("invite: owner/manager check failed", "team", team.Id, "user", caller.Id, "err", err)
				return apis.NewInternalServerError("permission check failed", err)
			}
			if !ok {
				return apis.NewForbiddenError("only team owners or managers may invite", nil)
			}

			target, err := e.App.FindRecordById("users", body.UserID)
			if err != nil {
				return apis.NewNotFoundError("user not found", err)
			}
			if target.GetBool("is_deleted") {
				return apis.NewBadRequestError("target user is deleted", nil)
			}
			if target.Id == caller.Id {
				return apis.NewBadRequestError("cannot invite yourself", nil)
			}

			// Default-gamertag check — the accept route will resolve the
			// recipient's default at acceptance time, so we fail fast here
			// rather than create an invite that can't be acted on.
			defaultTagID := target.GetString("default_gamertag")
			if defaultTagID == "" {
				return apis.NewBadRequestError("target user has no default gamertag set", nil)
			}
			defaultTag, err := e.App.FindRecordById("gamertags", defaultTagID)
			if err != nil {
				return apis.NewBadRequestError("target user's default gamertag is missing", err)
			}
			if defaultTag.GetString("status") == "blocked" {
				return apis.NewBadRequestError("target user's default gamertag is blocked", nil)
			}

			// Already-on-the-team guard.
			alreadyMember, err := teamperms.HasActiveMembership(e.App, target.Id, team.Id)
			if err != nil {
				e.App.Logger().Error("invite: membership check failed", "team", team.Id, "user", target.Id, "err", err)
				return apis.NewInternalServerError("membership check failed", err)
			}
			if alreadyMember {
				return apis.NewApiError(http.StatusConflict, "target user is already on the team", nil)
			}

			// Pending-duplicate guard. Returns the existing row if any —
			// the API caller can then act on that one (decide whether to
			// cancel + retry or just wait).
			pending, err := teamperms.FindPendingRequest(e.App, team.Id, target.Id)
			if err != nil {
				e.App.Logger().Error("invite: pending check failed", "team", team.Id, "user", target.Id, "err", err)
				return apis.NewInternalServerError("pending check failed", err)
			}
			if pending != nil {
				return apis.NewApiError(http.StatusConflict, "a pending request already exists for this team and user", nil)
			}

			// Materialize the request row.
			col, err := e.App.FindCollectionByNameOrId("team_membership_requests")
			if err != nil {
				return apis.NewInternalServerError("request collection missing", err)
			}
			req := core.NewRecord(col)
			req.Set("team", team.Id)
			req.Set("user", target.Id)
			req.Set("direction", "invited")
			req.Set("status", "pending")
			req.Set("initiated_by", caller.Id)
			if err := e.App.Save(req); err != nil {
				return apis.NewInternalServerError("failed to create invite", err)
			}

			// Side effects — notification + team_log. Both are best-effort:
			// log the error but keep the 201 because the request is the
			// source of truth and either can be re-derived if it ever drops.
			callerHandleID := caller.GetString("default_gamertag")
			callerHandle := lookupHandle(e.App, callerHandleID)
			callerName := caller.GetString("name")

			if err := notifications.Notify(e.App, target, notifications.NotifTypeTeamInvite, notifications.TeamInvitePayload{
				RequestID:         req.Id,
				TeamID:            team.Id,
				TeamName:          team.GetString("name"),
				TeamSlug:          team.GetString("slug"),
				InitiatedByID:     caller.Id,
				InitiatedByName:   callerName,
				InitiatedByHandle: callerHandle,
			}); err != nil {
				e.App.Logger().Error("invite: notification fan-out failed", "request", req.Id, "err", err)
			}

			if err := teamlog.Write(e.App, team, caller, teamlog.EventInviteCreated, target, nil, teamlog.InviteCreatedPayload{
				RequestID: req.Id,
			}); err != nil {
				e.App.Logger().Error("invite: team_log write failed", "request", req.Id, "err", err)
			}

			return e.JSON(http.StatusCreated, req)
		})
	})
}

// lookupHandle resolves a gamertag id to its `tag` field, returning "" on
// any miss. Used by the invite + join_request handlers to populate the
// notification payload with a human-readable display handle without
// failing the parent operation when the lookup errors.
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
