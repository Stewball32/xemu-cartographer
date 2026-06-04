package hooks

import (
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"

	"github.com/Stewball32/xemu-cartographer/internal/notifications"
	"github.com/Stewball32/xemu-cartographer/internal/roles"
	"github.com/Stewball32/xemu-cartographer/internal/teamlog"
	"github.com/Stewball32/xemu-cartographer/internal/teamperms"
)

func init() {
	register(registerUsersSoftDeletePIIHook)
}

// registerUsersSoftDeletePIIHook blanks PII when a user is soft-deleted
// (is_deleted flipped false → true). Keeps the id + username + isAdmin so
// roster history + admin-audit context survive; clears email, name, bio,
// location, avatar so a "deleted" account doesn't leak the person who used
// to own it. Stamps deleted_at to the current UTC time.
//
// Username is intentionally preserved: the users_username_immutable hook
// also blocks changes here, and downstream /u/[username]/ pages need a
// stable handle to look up the tombstoned record and render "[deleted
// user]" — there's no PII in the username itself.
//
// Idempotent: subsequent updates that arrive with is_deleted=true but the
// PII blanks already applied pass through untouched (the false→true
// transition gate skips them).
//
// Pairs with users.AuthRule "is_deleted = false" in schema/users.go — the
// rule blocks login, this hook scrubs the data.
//
// M23e + M8f cascade. After PII blanking, the hook also reaps the user's
// notifications + active membership state + role grants so nothing
// dangling addresses the tombstoned account:
//  1. Delete every `notifications` row where user = id. The bell + page
//     would render against a "[deleted user]" name otherwise; cheaper to
//     drop the rows than to redact each one.
//  2. Flip every pending `team_membership_requests` row where user = id
//     OR initiated_by = id to status=cancelled, then fan out the matching
//     `*_cancelled` notifications to the counterparties with
//     BySoftDelete=true so the UI can render an appropriate tombstone
//     copy. Also writes EventInviteCancelled / EventRequestCancelled
//     team_log rows with ViaSoftDelete=true.
//  3. Stamp left_at=now on every active roster row whose gamertag belongs
//     to the user. Writes EventMemberLeft with ViaSoftDelete=true. No
//     notification fan-out (the team page activity timeline carries the
//     leave event, and ownership/manager arithmetic isn't worth a per-
//     teammate alert).
//  4. (M8f) Revoke every user_roles row through roles.Revoke so the
//     audit_log carries a per-role ActionRoleRevoke entry tagged with the
//     soft-delete reason. The audit rows survive the role row deletion
//     because audit_log.target_id is unconstrained text.
//
// Cascade failures log + continue — the soft-delete itself is the
// load-bearing transition; trailing notifications/team_log/roster/role
// updates are best-effort.
func registerUsersSoftDeletePIIHook(app *pocketbase.PocketBase) {
	app.OnRecordUpdate("users").BindFunc(func(e *core.RecordEvent) error {
		wasDeleted := e.Record.Original().GetBool("is_deleted")
		isDeleted := e.Record.GetBool("is_deleted")
		if wasDeleted || !isDeleted {
			return e.Next()
		}

		// PII blanking. Email becomes a tombstone string keyed off the user
		// id so the column's unique constraint still holds without a real
		// inbox address. Avatar wipes the filename pointer (the file stays
		// on disk; janitor cleanup is out of scope here).
		e.Record.Set("email", "deleted+"+e.Record.Id+"@invalid.local")
		e.Record.Set("emailVisibility", false)
		e.Record.Set("verified", false)
		e.Record.Set("name", "")
		e.Record.Set("bio", "")
		e.Record.Set("location", "")
		e.Record.Set("avatar", "")
		e.Record.Set("default_gamertag", "")

		if deletedAt, err := types.ParseDateTime(time.Now().UTC()); err == nil {
			e.Record.Set("deleted_at", deletedAt)
		}

		// Cascade — run after PII but before returning. The order is:
		// notifications drop → pending requests cancel + fan-out → active
		// rosters left_at. None of these block the PII transition; we
		// log + continue on every error.
		cascadeSoftDelete(e.App, e.Record)

		return e.Next()
	})
}

// cascadeSoftDelete reaps a soft-deleted user's notifications, pending
// requests, and active roster rows. Best-effort: every step logs + moves
// on if the underlying read or write errors out.
func cascadeSoftDelete(app core.App, user *core.Record) {
	userID := user.Id

	// 1. Drop notifications addressed to this user.
	if notifs, err := app.FindRecordsByFilter(
		"notifications",
		"user = {:userID}",
		"-created", 0, 0,
		dbx.Params{"userID": userID},
	); err == nil {
		for _, n := range notifs {
			if err := app.Delete(n); err != nil {
				app.Logger().Error("soft-delete: drop notification failed", "id", n.Id, "err", err)
			}
		}
	} else {
		app.Logger().Error("soft-delete: notifications lookup failed", "user", userID, "err", err)
	}

	// 2. Cancel pending team_membership_requests in either role + fan out.
	pending, err := app.FindRecordsByFilter(
		"team_membership_requests",
		"status = \"pending\" && (user = {:userID} || initiated_by = {:userID})",
		"-created", 0, 0,
		dbx.Params{"userID": userID},
	)
	if err != nil {
		app.Logger().Error("soft-delete: pending requests lookup failed", "user", userID, "err", err)
	}
	for _, req := range pending {
		req.Set("status", "cancelled")
		req.Set("responded_by", userID)
		if err := app.Save(req); err != nil {
			app.Logger().Error("soft-delete: cancel request failed", "id", req.Id, "err", err)
			continue
		}

		team, terr := app.FindRecordById("teams", req.GetString("team"))
		if terr != nil {
			continue
		}
		direction := req.GetString("direction")
		// "user" perspective: the recipient of the action.
		//   invite-direction → user is the invitee; if they cancelled by
		//     deletion, notify the inviter.
		//   request-direction → user is the requester; if they cancelled
		//     by deletion, fan out to owners/managers.
		// "initiated_by" perspective: the originator.
		//   invite-direction → initiator deleted → notify the invitee.
		//   request-direction → initiator = user = deleted → fan out to
		//     owners/managers (same as above; the OR collapses cases).
		if direction == "invited" {
			// Notify whichever counterparty isn't the deleted user.
			notifyTargetID := req.GetString("initiated_by")
			if notifyTargetID == userID {
				notifyTargetID = req.GetString("user")
			}
			if notifyTargetID != "" && notifyTargetID != userID {
				if other, lerr := app.FindRecordById("users", notifyTargetID); lerr == nil {
					_ = notifications.Notify(app, other, notifications.NotifTypeTeamInviteCancelled, notifications.TeamInviteCancelledPayload{
						RequestID:    req.Id,
						TeamID:       team.Id,
						TeamName:     team.GetString("name"),
						TeamSlug:     team.GetString("slug"),
						BySoftDelete: true,
					})
				}
			}
			_ = teamlog.Write(app, team, nil, teamlog.EventInviteCancelled, user, nil, teamlog.InviteCancelledPayload{
				RequestID:     req.Id,
				ViaSoftDelete: true,
			})
		} else if direction == "requested" {
			owners, oerr := teamperms.OwnersAndManagers(app, team.Id)
			if oerr == nil {
				payload := notifications.TeamJoinRequestCancelledPayload{
					RequestID:    req.Id,
					TeamID:       team.Id,
					TeamName:     team.GetString("name"),
					TeamSlug:     team.GetString("slug"),
					BySoftDelete: true,
				}
				for _, owner := range owners {
					if owner.Id == userID {
						continue
					}
					_ = notifications.Notify(app, owner, notifications.NotifTypeTeamJoinRequestCancelled, payload)
				}
			}
			_ = teamlog.Write(app, team, nil, teamlog.EventRequestCancelled, user, nil, teamlog.RequestCancelledPayload{
				RequestID:     req.Id,
				ViaSoftDelete: true,
			})
		}
	}

	// 3. Stamp left_at on every active roster row whose gamertag belongs
	// to this user. Walks the rosters.gamertag.user relation chain.
	rosters, err := app.FindRecordsByFilter(
		"rosters",
		"left_at = null && gamertag.user = {:userID}",
		"-joined_at", 0, 0,
		dbx.Params{"userID": userID},
	)
	if err != nil {
		app.Logger().Error("soft-delete: rosters lookup failed", "user", userID, "err", err)
	} else {
		leftAt, _ := types.ParseDateTime(time.Now().UTC())
		for _, row := range rosters {
			row.Set("left_at", leftAt)
			if err := app.Save(row); err != nil {
				app.Logger().Error("soft-delete: stamp left_at failed", "roster", row.Id, "err", err)
				continue
			}
			team, terr := app.FindRecordById("teams", row.GetString("team"))
			if terr != nil {
				continue
			}
			var gt *core.Record
			if id := row.GetString("gamertag"); id != "" {
				if rec, gerr := app.FindRecordById("gamertags", id); gerr == nil {
					gt = rec
				}
			}
			var gamertag string
			if gt != nil {
				gamertag = gt.GetString("tag")
			}
			_ = teamlog.Write(app, team, nil, teamlog.EventMemberLeft, user, gt, teamlog.MemberLeftPayload{
				Gamertag:      gamertag,
				ViaSoftDelete: true,
			})
		}
	}

	// 4. (M8f) Revoke every user_roles row. roles.Revoke writes an
	// ActionRoleRevoke audit row per slug with reason="user soft-deleted"
	// so the admin timeline shows the cascade; the audit_log target_id is
	// unconstrained text, so rows survive the role row delete.
	slugs, err := roles.Slugs(app, userID)
	if err != nil {
		app.Logger().Error("soft-delete: roles lookup failed", "user", userID, "err", err)
		return
	}
	for _, slug := range slugs {
		if err := roles.Revoke(app, userID, slug, nil, "user soft-deleted"); err != nil {
			app.Logger().Error("soft-delete: role revoke failed", "user", userID, "slug", slug, "err", err)
		}
	}
}
