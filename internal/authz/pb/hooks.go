package pb

import (
	"fmt"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/xemu-cartographer/xemu-cartographer/internal/audit"
)

// bindingFields maps the collections an api_tokens row can be bound to onto
// the api_tokens field holding the reference: gamertags (PD-8 station
// binding, multiple) and users (the key's linked user, single).
var bindingFields = map[string]string{
	"gamertags": "gamertags",
	"users":     "user",
}

// RegisterHooks installs the adapter's record hooks on app. NewDeps calls it
// once per adapter (the ephemeral adapters depsOr / withApp build do not):
//
//   - roles create / update / delete ⇒ InvalidateRoles, so an edited level or
//     scope list is live on the next request rather than after rolesCacheTTL
//     (§6.2).
//   - gamertags / users delete ⇒ every api_tokens row bound to the record is
//     revoked first, inside the delete's transaction. PocketBase would
//     otherwise just unset the relation id, and a station-bound key with an
//     empty gamertags relation is an unbound one that serves every profile.
//   - users soft-delete (is_deleted / deleted_at flipping on) ⇒ the same
//     revoke, because this app's deletion path is the soft one and a key
//     linked to a soft-deleted user must not outlive the user's own JWT.
func RegisterHooks(app core.App, d *PBDeps) {
	if app == nil || d == nil {
		return
	}
	invalidate := func(e *core.RecordEvent) error {
		d.InvalidateRoles()
		return e.Next()
	}
	app.OnRecordAfterCreateSuccess("roles").BindFunc(invalidate)
	app.OnRecordAfterUpdateSuccess("roles").BindFunc(invalidate)
	app.OnRecordAfterDeleteSuccess("roles").BindFunc(invalidate)

	app.OnRecordDeleteExecute("gamertags").BindFunc(revokeBoundTokensOnDelete)
	app.OnRecordDeleteExecute("users").BindFunc(revokeBoundTokensOnDelete)
	app.OnRecordUpdateExecute("users").BindFunc(revokeBoundTokensOnSoftDelete)
}

// softDeleted reports whether a users row carries either soft-delete marker.
func softDeleted(rec *core.Record) bool {
	return rec != nil && (rec.GetBool("is_deleted") || !rec.GetDateTime("deleted_at").IsZero())
}

// revokeBoundTokensOnSoftDelete is the OnRecordUpdateExecute handler for
// users: when the save turns a live row into a soft-deleted one, revoke the
// keys linked to it inside the update's transaction. Other updates (bans
// included — they are reversible, a revoke is not) pass straight through.
func revokeBoundTokensOnSoftDelete(e *core.RecordEvent) error {
	if e.Record == nil || !softDeleted(e.Record) || softDeleted(e.Record.Original()) {
		return e.Next()
	}
	original := e.App
	err := e.App.RunInTransaction(func(txApp core.App) error {
		e.App = txApp
		reason := "binding soft-deleted: users/" + e.Record.Id
		if err := revokeTokensBoundTo(txApp, bindingFields["users"], e.Record, reason); err != nil {
			return err
		}
		return e.Next()
	})
	e.App = original
	return err
}

// revokeBoundTokensOnDelete is the OnRecordDeleteExecute handler for the
// bindingFields collections: revoke the referencing api_tokens rows, then
// let the delete run, all in one transaction (the same e.App swap
// PocketBase's own cascade handlers use) so neither half lands without the
// other.
func revokeBoundTokensOnDelete(e *core.RecordEvent) error {
	if e.Record == nil || e.Record.Collection() == nil {
		return e.Next()
	}
	field, ok := bindingFields[e.Record.Collection().Name]
	if !ok {
		return e.Next()
	}
	original := e.App
	err := e.App.RunInTransaction(func(txApp core.App) error {
		e.App = txApp
		reason := "binding deleted: " + e.Record.Collection().Name + "/" + e.Record.Id
		if err := revokeTokensBoundTo(txApp, field, e.Record, reason); err != nil {
			return err
		}
		return e.Next()
	})
	e.App = original
	return err
}

// revokeTokensBoundTo flags every live api_tokens row whose field references
// rec as revoked (revoked_at now, no revoked_by — there is no actor) and
// writes the ActionTokenRevoke audit row carrying reason.
func revokeTokensBoundTo(app core.App, field string, rec *core.Record, reason string) error {
	var rows []*core.Record
	var err error
	if field == "gamertags" {
		// Multi relation: match any element of the stored id list.
		rows, err = app.FindRecordsByFilter(
			"api_tokens",
			"gamertags:each ?= {:id} && revoked = false",
			"", 0, 0,
			dbx.Params{"id": rec.Id},
		)
	} else {
		rows, err = app.FindRecordsByFilter(
			"api_tokens",
			field+" = {:id} && revoked = false",
			"", 0, 0,
			dbx.Params{"id": rec.Id},
		)
	}
	if err != nil {
		return fmt.Errorf("authz: api_tokens bound to %s/%s: %w", rec.Collection().Name, rec.Id, err)
	}
	for _, row := range rows {
		row.Set("revoked", true)
		row.Set("revoked_at", time.Now().UTC())
		row.Set("revoked_by", "")
		if err := app.SaveNoValidate(row); err != nil {
			return fmt.Errorf("authz: revoke api_tokens %s: %w", row.GetString("kid"), err)
		}
		payload := audit.TokenRevokePayload{
			Kid:    row.GetString("kid"),
			Reason: reason,
			Actor:  "internal:hook",
		}
		if err := audit.WriteRef(app, nil, audit.ActionTokenRevoke, "api_tokens", row.Id, payload); err != nil {
			return fmt.Errorf("authz: audit revoke of %s: %w", row.GetString("kid"), err)
		}
	}
	return nil
}
