package hooks

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/audit"
	"github.com/Stewball32/xemu-cartographer/internal/roles"
)

func init() {
	register(registerGamertagsStatusTransitionsHook)
}

const (
	gtStatusApproved = "approved"
	gtStatusAllowed  = "allowed"
	gtStatusBlocked  = "blocked"
)

// registerGamertagsStatusTransitionsHook owns the M22b status state machine
// for gamertags: defaults new rows to "allowed", auto-downgrades approved
// rows to "allowed" when the owner edits the `tag` field, enforces
// admin-only direct status writes, and emits an audit_log row through the
// internal/audit helper for every legitimate moderation transition.
//
// Hook fan-out:
//
//   - OnRecordCreate: ensure status is non-empty (defaults to "allowed").
//     Fires for both API and programmatic saves; no audit row written here —
//     the row's existence + created timestamp tell the whole story.
//
//   - OnRecordUpdateRequest: API-only, so we have e.Auth for the actor.
//     Handles the auto-downgrade (approved row whose tag was edited), the
//     non-admin status-write gate, and the audit row emission. Programmatic
//     updates (migration backfill, other server-side code) skip this hook
//     entirely; the M22b migration emits its own synthetic audit rows.
//
// The migration backfill (schema/gamertags.go) calls `app.Save(row)` after
// setting status from empty → "blocked"/"allowed". OnRecordUpdate fires for
// that path, NOT OnRecordUpdateRequest, so the migration is invisible here.
// If we ever audit on OnRecordUpdate too, gate on `prev != ""` to skip the
// initial assignment.
func registerGamertagsStatusTransitionsHook(app *pocketbase.PocketBase) {
	app.OnRecordCreate("gamertags").BindFunc(func(e *core.RecordEvent) error {
		if e.Record.GetString("status") == "" {
			e.Record.Set("status", gtStatusAllowed)
		}
		return e.Next()
	})

	app.OnRecordUpdateRequest("gamertags").BindFunc(func(e *core.RecordRequestEvent) error {
		prev := e.Record.Original().GetString("status")
		next := e.Record.GetString("status")
		prevTag := e.Record.Original().GetString("tag")
		newTag := e.Record.GetString("tag")
		tagChanged := prevTag != newTag

		actorIsAdmin := roles.IsAdminAuth(e.App, e.Auth)

		// Auto-downgrade: owner edited the tag of an approved row. Force
		// status back to allowed so the next admin review re-evaluates the
		// new string. Runs whether or not the client also tried to change
		// status, as long as the prior status was approved.
		if prev == gtStatusApproved && tagChanged && next == gtStatusApproved {
			e.Record.Set("status", gtStatusAllowed)
			next = gtStatusAllowed
			if err := audit.Write(e.App, e.Auth, audit.ActionEdit, e.Record, audit.EditPayload{
				Field:      "tag",
				PrevValue:  prevTag,
				NewValue:   newTag,
				PrevStatus: prev,
				NewStatus:  next,
			}); err != nil {
				e.App.Logger().Error("M22b: audit ActionEdit failed", "id", e.Record.Id, "err", err)
			}
			return e.Next()
		}

		// Status didn't change in any other way — nothing to audit. Pass.
		if prev == next {
			return e.Next()
		}

		// Non-admin trying to flip status directly. Reject the whole update
		// rather than silently revert; PB rules already pass writes through
		// here for the owner case, so the gate has to live in the hook.
		if !actorIsAdmin {
			return apis.NewBadRequestError("gamertag status changes require admin", nil)
		}

		// Admin-driven transition. Pick the action enum based on the new
		// status; unknown transitions fall through unaudited (no enum yet).
		switch next {
		case gtStatusBlocked:
			if err := audit.Write(e.App, e.Auth, audit.ActionBlock, e.Record, audit.BlockPayload{
				PrevStatus: prev,
			}); err != nil {
				e.App.Logger().Error("M22b: audit ActionBlock failed", "id", e.Record.Id, "err", err)
			}
		case gtStatusAllowed:
			if prev == gtStatusBlocked {
				if err := audit.Write(e.App, e.Auth, audit.ActionUnblock, e.Record, audit.UnblockPayload{}); err != nil {
					e.App.Logger().Error("M22b: audit ActionUnblock failed", "id", e.Record.Id, "err", err)
				}
			}
		case gtStatusApproved:
			if err := audit.Write(e.App, e.Auth, audit.ActionApprove, e.Record, audit.ApprovePayload{
				PrevStatus: prev,
			}); err != nil {
				e.App.Logger().Error("M22b: audit ActionApprove failed", "id", e.Record.Id, "err", err)
			}
		}

		return e.Next()
	})
}
