package hooks

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/audit"
	"github.com/Stewball32/xemu-cartographer/internal/teamlog"
)

func init() {
	register(registerTeamsStatusTransitionsHook)
}

const (
	tmStatusApproved = "approved"
	tmStatusAllowed  = "allowed"
	tmStatusBlocked  = "blocked"
)

// registerTeamsStatusTransitionsHook owns the M22d state machine for teams.
// Structurally parallel to the gamertags hook from M22b, with one extra
// concern: team renames are always audited (ActionRename) so the team page
// "formerly known as" history view can read them back. ActionEdit still
// fires only for the auto-downgrade case where renaming an approved team
// flips status to allowed — that's a moderation event, distinct from the
// rename event itself.
//
// Hook fan-out:
//
//   - OnRecordCreate: ensure status is non-empty (defaults to "allowed").
//     Fires for both API and programmatic saves; no audit row written here.
//
//   - OnRecordUpdateRequest: API-only, so we have e.Auth for the actor.
//     Handles rename auditing (every name change), the auto-downgrade
//     (approved team whose name was edited), the non-admin status-write
//     gate, and the status-transition audit emission. Programmatic updates
//     (migration backfill) skip this hook entirely.
func registerTeamsStatusTransitionsHook(app *pocketbase.PocketBase) {
	app.OnRecordCreate("teams").BindFunc(func(e *core.RecordEvent) error {
		if e.Record.GetString("status") == "" {
			e.Record.Set("status", tmStatusAllowed)
		}
		return e.Next()
	})

	app.OnRecordUpdateRequest("teams").BindFunc(func(e *core.RecordRequestEvent) error {
		prev := e.Record.Original().GetString("status")
		next := e.Record.GetString("status")
		prevName := e.Record.Original().GetString("name")
		newName := e.Record.GetString("name")
		nameChanged := prevName != newName

		actorIsAdmin := e.Auth != nil && (e.Auth.IsSuperuser() || e.Auth.GetBool("isAdmin"))

		// Always audit name changes — even on non-approved rows + admin renames
		// — so the team page can show every name the team has carried.
		// M23c twin-writes the rename to team_log so the public team page
		// can read it without admin-only audit_log access.
		if nameChanged {
			if err := audit.Write(e.App, e.Auth, audit.ActionRename, e.Record, audit.RenamePayload{
				PrevName: prevName,
				NewName:  newName,
				ByAdmin:  actorIsAdmin,
			}); err != nil {
				e.App.Logger().Error("M22d: audit ActionRename failed", "id", e.Record.Id, "err", err)
			}
			if err := teamlog.Write(e.App, e.Record, e.Auth, teamlog.EventTeamRenamed, nil, nil, teamlog.TeamRenamedPayload{
				PrevName: prevName,
				NewName:  newName,
				ByAdmin:  actorIsAdmin,
			}); err != nil {
				e.App.Logger().Error("M23c: team_log EventTeamRenamed failed", "id", e.Record.Id, "err", err)
			}
		}

		// Auto-downgrade: owner renamed an approved team. Force status back
		// to allowed and emit ActionEdit so the moderation queue picks it
		// up alongside the rename event.
		if prev == tmStatusApproved && nameChanged && next == tmStatusApproved {
			e.Record.Set("status", tmStatusAllowed)
			next = tmStatusAllowed
			if err := audit.Write(e.App, e.Auth, audit.ActionEdit, e.Record, audit.EditPayload{
				Field:      "name",
				PrevValue:  prevName,
				NewValue:   newName,
				PrevStatus: prev,
				NewStatus:  next,
			}); err != nil {
				e.App.Logger().Error("M22d: audit ActionEdit failed", "id", e.Record.Id, "err", err)
			}
			return e.Next()
		}

		// No status change beyond the rename path? Done.
		if prev == next {
			return e.Next()
		}

		// Non-admin trying to flip status directly. Reject the whole update —
		// PB rules already let owners through, so status enforcement has to
		// live in the hook.
		if !actorIsAdmin {
			return apis.NewBadRequestError("team status changes require admin", nil)
		}

		switch next {
		case tmStatusBlocked:
			if err := audit.Write(e.App, e.Auth, audit.ActionBlock, e.Record, audit.BlockPayload{
				PrevStatus: prev,
			}); err != nil {
				e.App.Logger().Error("M22d: audit ActionBlock failed", "id", e.Record.Id, "err", err)
			}
		case tmStatusAllowed:
			if prev == tmStatusBlocked {
				if err := audit.Write(e.App, e.Auth, audit.ActionUnblock, e.Record, audit.UnblockPayload{}); err != nil {
					e.App.Logger().Error("M22d: audit ActionUnblock failed", "id", e.Record.Id, "err", err)
				}
			}
		case tmStatusApproved:
			if err := audit.Write(e.App, e.Auth, audit.ActionApprove, e.Record, audit.ApprovePayload{
				PrevStatus: prev,
			}); err != nil {
				e.App.Logger().Error("M22d: audit ActionApprove failed", "id", e.Record.Id, "err", err)
			}
		}

		// M23c twin-write to team_log so the team page can render the
		// transition. audit_log keeps its admin-only consumer; team_log is
		// the user-facing surface.
		if err := teamlog.Write(e.App, e.Record, e.Auth, teamlog.EventTeamStatusChanged, nil, nil, teamlog.TeamStatusChangedPayload{
			PrevStatus: prev,
			NewStatus:  next,
		}); err != nil {
			e.App.Logger().Error("M23c: team_log EventTeamStatusChanged failed", "id", e.Record.Id, "err", err)
		}

		return e.Next()
	})
}
