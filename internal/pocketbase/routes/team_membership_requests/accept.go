package team_membership_requests

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
		// POST /api/team-membership-requests/{id}/accept — no body.
		//
		// For direction=invited: the invitee accepts. Caller must be the
		//   request's `user`. Materializes a roster row using the
		//   invitee's current default_gamertag.
		// For direction=requested: an owner/manager approves. Caller must
		//   own/manage the team. Materializes a roster row using the
		//   requester's current default_gamertag.
		//
		// Default-gamertag resolution happens at accept time per the M23
		// "default_gamertag is the social handle" decision — invites have
		// no gamertag column; requests don't pre-pick one either. If the
		// recipient/requester has since changed or deleted their default,
		// the accept route fails with 400.
		//
		// Status transitions: 404 / 400 / 409 if any guard fails.
		// On success: 200 with the closed request row. Side effects —
		// roster row, accept notification to other party, EventMemberAdded
		// team_log row — all best-effort logging on failure.
		Group.POST("/{id}/accept", func(e *core.RequestEvent) error {
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
			if team.GetString("status") == "blocked" {
				return apis.NewBadRequestError("team is blocked", nil)
			}

			subjectUser, err := e.App.FindRecordById("users", req.GetString("user"))
			if err != nil {
				return apis.NewInternalServerError("subject user lookup failed", err)
			}
			if subjectUser.GetBool("is_deleted") {
				return apis.NewBadRequestError("subject user is deleted", nil)
			}

			direction := req.GetString("direction")
			switch direction {
			case "invited":
				// Invitee accepts. Caller must equal the request user.
				if caller.Id != subjectUser.Id {
					return apis.NewForbiddenError("only the invitee may accept this invite", nil)
				}
			case "requested":
				// Owner/manager approves. Caller must own/manage the team.
				ok, err := teamperms.IsOwnerOrManager(e.App, caller.Id, team.Id)
				if err != nil {
					return apis.NewInternalServerError("permission check failed", err)
				}
				if !ok {
					return apis.NewForbiddenError("only an owner or manager may approve this request", nil)
				}
			default:
				return apis.NewInternalServerError("unknown request direction", nil)
			}

			// Resolve subject's default_gamertag at accept time. This is
			// the point where the M23 decision pays off — we don't have
			// to chase a gamertag picker through the UI.
			defaultTagID := subjectUser.GetString("default_gamertag")
			if defaultTagID == "" {
				return apis.NewBadRequestError("subject user has no default gamertag", nil)
			}
			defaultTag, err := e.App.FindRecordById("gamertags", defaultTagID)
			if err != nil {
				return apis.NewBadRequestError("subject's default gamertag is missing", err)
			}
			if defaultTag.GetString("status") == "blocked" {
				return apis.NewBadRequestError("subject's default gamertag is blocked", nil)
			}

			// Already-on-the-team belt-and-suspenders (the original
			// create-route guard checked this, but state may have
			// changed between request creation and acceptance).
			alreadyMember, err := teamperms.HasActiveMembership(e.App, subjectUser.Id, team.Id)
			if err != nil {
				return apis.NewInternalServerError("membership check failed", err)
			}
			if alreadyMember {
				return apis.NewApiError(http.StatusConflict, "subject user already on the team", nil)
			}

			// Create the roster row + close the request in the same flow.
			// PocketBase's app.Save doesn't expose explicit transactions
			// here, but the two writes are independent — if the second
			// fails the first is fine to keep (an extra roster row only
			// reflects the actual joined state) and a retry of the accept
			// will short-circuit on the already-member guard above.
			rostersCol, err := e.App.FindCollectionByNameOrId("rosters")
			if err != nil {
				return apis.NewInternalServerError("rosters collection missing", err)
			}
			roster := core.NewRecord(rostersCol)
			roster.Set("team", team.Id)
			roster.Set("gamertag", defaultTag.Id)
			roster.Set("is_owner", false)
			roster.Set("is_manager", false)
			if joinedAt, parseErr := types.ParseDateTime(time.Now().UTC()); parseErr == nil {
				roster.Set("joined_at", joinedAt)
			}
			if err := e.App.Save(roster); err != nil {
				return apis.NewInternalServerError("failed to create roster row", err)
			}

			req.Set("status", "accepted")
			req.Set("responded_by", caller.Id)
			if err := e.App.Save(req); err != nil {
				e.App.Logger().Error("accept: failed to close request after roster create", "request", req.Id, "err", err)
				return apis.NewInternalServerError("failed to close request", err)
			}

			// Notify the other party. For invited: the inviter (request's
			// initiated_by). For requested: the requester (same field —
			// initiated_by always names who opened the request).
			otherPartyID := req.GetString("initiated_by")
			if direction == "requested" {
				// Requester opened; notify them their request was accepted.
				// initiated_by here is the requester, same as subjectUser.
				// We still notify because the notification carries the
				// acceptor's identity which the requester doesn't otherwise
				// see in their settings.
				otherPartyID = subjectUser.Id
			}
			if otherPartyID != "" && otherPartyID != caller.Id {
				other, lookupErr := e.App.FindRecordById("users", otherPartyID)
				if lookupErr == nil {
					acceptorHandleID := caller.GetString("default_gamertag")
					acceptorHandle := lookupHandle(e.App, acceptorHandleID)
					acceptorName := caller.GetString("name")
					if direction == "invited" {
						_ = notifications.Notify(e.App, other, notifications.NotifTypeTeamInviteAccepted, notifications.TeamInviteAcceptedPayload{
							RequestID:      req.Id,
							TeamID:         team.Id,
							TeamName:       team.GetString("name"),
							TeamSlug:       team.GetString("slug"),
							AcceptorID:     caller.Id,
							AcceptorName:   acceptorName,
							AcceptorHandle: acceptorHandle,
						})
					} else {
						_ = notifications.Notify(e.App, other, notifications.NotifTypeTeamJoinRequestAccepted, notifications.TeamJoinRequestAcceptedPayload{
							RequestID:      req.Id,
							TeamID:         team.Id,
							TeamName:       team.GetString("name"),
							TeamSlug:       team.GetString("slug"),
							AcceptorID:     caller.Id,
							AcceptorName:   acceptorName,
							AcceptorHandle: acceptorHandle,
						})
					}
				}
			}

			// team_log: one EventMemberAdded row with the route flag so
			// the team page can render "joined via invite/request". We
			// deliberately don't write a paired *_accepted event — the
			// roster row's existence is the canonical join signal.
			if err := teamlog.Write(e.App, team, caller, teamlog.EventMemberAdded, subjectUser, defaultTag, teamlog.MemberAddedPayload{
				Via:       direction, // "invited" | "requested"
				RequestID: req.Id,
				Gamertag:  defaultTag.GetString("tag"),
			}); err != nil {
				e.App.Logger().Error("accept: team_log EventMemberAdded failed", "request", req.Id, "err", err)
			}

			return e.JSON(http.StatusOK, req)
		})
	})
}

// lookupHandle resolves a gamertag id to its `tag` field, returning "" on
// any miss. Mirrors the helper in routes/teams/invite.go — duplicated
// because the two packages can't import each other without a cycle.
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
