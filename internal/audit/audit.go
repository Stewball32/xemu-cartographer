package audit

import (
	"encoding/json"
	"fmt"

	"github.com/pocketbase/pocketbase/core"
)

// Write records an audit_log row for a state change on target. actor may be
// nil for system-triggered events (migrations, cron jobs, backfills). payload
// is JSON-marshaled into audit_log.payload_json; pass nil to leave the column
// empty for actions whose existence is the whole story (e.g. ActionApprove
// without a reason field).
//
// Returns the error from the JSON marshal or the underlying app.Save. Callers
// typically log + continue rather than fail the parent operation, but the
// helper returns the error so the choice stays at the call site.
//
// Use WriteRef when the target record has already been deleted (post-delete
// hooks, synthetic backfill rows) — Write derefs target.Collection() and
// target.Id, which need a live record.
func Write(app core.App, actor *core.Record, action Action, target *core.Record, payload any) error {
	if target == nil {
		return fmt.Errorf("audit.Write: target record is nil; use WriteRef for synthetic or post-delete entries")
	}
	return WriteRef(app, actor, action, target.Collection().Name, target.Id, payload)
}

// WriteRef is the lower-level form that takes the target collection name +
// id directly. Use this when the target record is no longer available
// (post-delete logging) or when synthesizing rows during a migration.
func WriteRef(app core.App, actor *core.Record, action Action, collection, id string, payload any) error {
	if collection == "" || id == "" {
		return fmt.Errorf("audit.WriteRef: collection and id are required (got %q / %q)", collection, id)
	}
	if action == "" {
		return fmt.Errorf("audit.WriteRef: action is required")
	}

	auditCol, err := app.FindCollectionByNameOrId("audit_log")
	if err != nil {
		return fmt.Errorf("audit.WriteRef: lookup audit_log collection: %w", err)
	}

	rec := core.NewRecord(auditCol)
	if actor != nil && actor.Collection() != nil && actor.Collection().Name == "users" {
		// The audit_log.actor field is a relation to the users collection.
		// PB superusers live in _superusers and would fail the relation
		// lookup if passed here; treat them as nil (system-triggered) and
		// rely on the audit_log row's existence + action + target to carry
		// the rest of the context.
		rec.Set("actor", actor.Id)
	}
	rec.Set("action", string(action))
	rec.Set("target_collection", collection)
	rec.Set("target_id", id)

	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("audit.WriteRef: marshal payload for action %q: %w", action, err)
		}
		rec.Set("payload_json", string(b))
	}

	if err := app.Save(rec); err != nil {
		return fmt.Errorf("audit.WriteRef: save audit row for action %q: %w", action, err)
	}
	return nil
}
