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
		// POST /api/teams/{teamId}/join-requests — caller asks to join.
		// No body — the caller's default_gamertag is the implicit join key
		// (resolved server-side; the M23 plan keeps gamertag picking out
		// of the user-facing path per the "default_gamertag is the social
		// handle" decision).
		//
		// Guards:
		//   400 — missing teamId; team is blocked; caller has no default
		//         gamertag or it's blocked
		//   404 — team doesn't exist
		//   409 — caller is already on the team, OR a pending request for
		//         (team, caller) already exists in either direction
		//
		// On success: 201 with the created team_membership_requests row;
		// one notification per active owner/manager; team_log
		// EventRequestCreated.
		Group.POST("/{teamId}/join-requests", func(e *core.RequestEvent) error {
			teamID := e.Request.PathValue("teamId")
			if teamID == "" {
				return apis.NewBadRequestError("teamId is required", nil)
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
				return apis.NewBadRequestError("team is blocked; join requests disabled", nil)
			}

			defaultTagID := caller.GetString("default_gamertag")
			if defaultTagID == "" {
				return apis.NewBadRequestError("set a default gamertag before requesting to join a team", nil)
			}
			defaultTag, err := e.App.FindRecordById("gamertags", defaultTagID)
			if err != nil {
				return apis.NewBadRequestError("default gamertag is missing", err)
			}
			if defaultTag.GetString("status") == "blocked" {
				return apis.NewBadRequestError("default gamertag is blocked", nil)
			}

			alreadyMember, err := teamperms.HasActiveMembership(e.App, caller.Id, team.Id)
			if err != nil {
				e.App.Logger().Error("join_request: membership check failed", "team", team.Id, "user", caller.Id, "err", err)
				return apis.NewInternalServerError("membership check failed", err)
			}
			if alreadyMember {
				return apis.NewApiError(http.StatusConflict, "you are already on this team", nil)
			}

			pending, err := teamperms.FindPendingRequest(e.App, team.Id, caller.Id)
			if err != nil {
				e.App.Logger().Error("join_request: pending check failed", "team", team.Id, "user", caller.Id, "err", err)
				return apis.NewInternalServerError("pending check failed", err)
			}
			if pending != nil {
				return apis.NewApiError(http.StatusConflict, "a pending request already exists for this team", nil)
			}

			col, err := e.App.FindCollectionByNameOrId("team_membership_requests")
			if err != nil {
				return apis.NewInternalServerError("request collection missing", err)
			}
			req := core.NewRecord(col)
			req.Set("team", team.Id)
			req.Set("user", caller.Id)
			req.Set("direction", "requested")
			req.Set("status", "pending")
			req.Set("initiated_by", caller.Id)
			if err := e.App.Save(req); err != nil {
				return apis.NewInternalServerError("failed to create join request", err)
			}

			// Notification fan-out — one row per active owner/manager. An
			// empty owner list (a team with no owner/manager — should be
			// rare but possible if someone left) means the request lands
			// but nobody can act on it; admins are the only resolution
			// path. Log the situation so it shows up in operator metrics.
			callerHandle := defaultTag.GetString("tag")
			callerName := caller.GetString("name")

			owners, err := teamperms.OwnersAndManagers(e.App, team.Id)
			if err != nil {
				e.App.Logger().Error("join_request: owners lookup failed", "team", team.Id, "err", err)
			} else if len(owners) == 0 {
				e.App.Logger().Warn("join_request: team has no active owner/manager", "team", team.Id, "request", req.Id)
			} else {
				payload := notifications.TeamJoinRequestPayload{
					RequestID:       req.Id,
					TeamID:          team.Id,
					TeamName:        team.GetString("name"),
					TeamSlug:        team.GetString("slug"),
					RequesterID:     caller.Id,
					RequesterName:   callerName,
					RequesterHandle: callerHandle,
				}
				for _, owner := range owners {
					if err := notifications.Notify(e.App, owner, notifications.NotifTypeTeamJoinRequest, payload); err != nil {
						e.App.Logger().Error("join_request: notify owner failed", "owner", owner.Id, "err", err)
					}
				}
			}

			if err := teamlog.Write(e.App, team, caller, teamlog.EventRequestCreated, caller, defaultTag, teamlog.RequestCreatedPayload{
				RequestID: req.Id,
			}); err != nil {
				e.App.Logger().Error("join_request: team_log write failed", "request", req.Id, "err", err)
			}

			return e.JSON(http.StatusCreated, req)
		})
	})
}
