package schema

import (
	"fmt"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/audit"
)

// registerGamertagsCollection creates the gamertags collection — one row per
// (user, tag) combo. A user can own multiple tags ("Stewball32" / "Stewball"
// / "Stewie"); the default pick lives on users.default_gamertag.
//
// `sanitized` is the lowercased + trimmed mirror of `tag`, maintained by the
// gamertags_sanitize hook. Uniqueness lives on (user, sanitized) so that case
// variants ("Stewie" vs "STEWIE") can't dupe — the scraper-side lookup that
// lands in M9+ also keys off `sanitized` so player-name matching is
// case-insensitive without needing a `LOWER(tag)` query at hit time.
//
// `status` is the 4-state moderation enum landed in M22b (was a bare
// `blocked: bool` in M7). Values: approved / allowed / pending / blocked.
//
//   - approved: admin-vetted, owner cannot edit (M22c admin queue marks this).
//   - allowed:  default for new tags — visible to others, owner can edit
//     freely; admin hasn't promoted to approved or flagged.
//   - pending:  flagged for admin review (M22e reserved-name pre-list or
//     community report). Visibility deferred; no transitions in 22b.
//   - blocked:  admin has blocked. Owner cannot edit or delete, and the
//     (user, sanitized) unique index prevents re-adding the same string.
//
// State transitions are gated + audited by the gamertags_status_transitions
// hook ([internal/pocketbase/hooks/gamertags_status_transitions.go]): owner
// edits to `tag` on an approved row auto-downgrade to allowed for re-check;
// every other transition requires admin and writes an audit_log row through
// `audit.Write`.
//
// Registration order is controlled by identity.go (not init()) because
// rosters depends on teams and users depends on gamertags.
func registerGamertagsCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "gamertags") {
		if err := migrateGamertagsBlockedToStatus(app); err != nil {
			return err
		}
		return reconcileGamertagsRules(app)
	}

	collection := core.NewBaseCollection("gamertags")

	usersCol, err := requireCollection(app, "users")
	if err != nil {
		return err
	}

	collection.Fields.Add(
		&core.RelationField{
			Name:          "user",
			Required:      true,
			CollectionId:  usersCol.Id,
			MaxSelect:     1,
			CascadeDelete: false,
		},
		&core.TextField{
			Name:        "tag",
			Required:    true,
			Min:         1,
			Max:         12,
			Presentable: true,
		},
		&core.TextField{
			Name: "sanitized",
			Min:  1,
			Max:  12,
		},
		&core.SelectField{
			Name:   "status",
			Values: gamertagsStatusValues,
		},
		&core.AutodateField{
			Name:     "created",
			OnCreate: true,
		},
		&core.AutodateField{
			Name:     "updated",
			OnCreate: true,
			OnUpdate: true,
		},
	)

	collection.AddIndex("idx_gamertags_user_sanitized_unique", true, "user, sanitized", "")
	collection.AddIndex("idx_gamertags_sanitized", false, "sanitized", "")

	setGamertagsRules(collection)

	return app.Save(collection)
}

// gamertagsStatusValues is the SelectField allow-list for `status`. Keep in
// sync with the const block in [internal/pocketbase/hooks/gamertags_status_transitions.go].
var gamertagsStatusValues = []string{"approved", "allowed", "pending", "blocked"}

// migrateGamertagsBlockedToStatus is the M22b one-shot migration that swaps
// the M7 `blocked: bool` field for the 4-state `status` SelectField. Three
// stages, all idempotent so a partial run resumes cleanly on the next boot:
//
//  1. Add `status` SelectField if missing.
//  2. For every row with empty status, set status from blocked (blocked=true
//     → "blocked", else "allowed") and emit a synthetic audit_log row with
//     actor=nil for backfilled blocks so the moderation trail starts with the
//     existing block events recorded.
//  3. Drop the `blocked` field once every row has a non-empty status.
//
// Skips entirely on a fresh install (`registerGamertagsCollection`'s
// collection-exists check funnels here only when the collection is already
// present from a prior boot). Skips a no-op on subsequent boots once the
// migration has run (hasStatus && !hasBlocked == "fully migrated").
//
// The backfill calls `app.Save(row)`, which fires `OnRecordUpdate` hooks. The
// status-transitions hook ignores transitions where the prior status is
// empty, so this initial assignment is not double-audited.
func migrateGamertagsBlockedToStatus(app *pocketbase.PocketBase) error {
	col, err := app.FindCollectionByNameOrId("gamertags")
	if err != nil {
		return err
	}

	hasBlocked := col.Fields.GetByName("blocked") != nil
	hasStatus := col.Fields.GetByName("status") != nil

	if hasStatus && !hasBlocked {
		return nil
	}

	if !hasStatus {
		col.Fields.Add(&core.SelectField{
			Name:   "status",
			Values: gamertagsStatusValues,
		})
		if err := app.Save(col); err != nil {
			return fmt.Errorf("M22b migration: add status field: %w", err)
		}
	}

	rows, err := app.FindAllRecords("gamertags")
	if err != nil {
		return fmt.Errorf("M22b migration: load gamertags: %w", err)
	}
	for _, row := range rows {
		if row.GetString("status") != "" {
			continue
		}
		newStatus := "allowed"
		if hasBlocked && row.GetBool("blocked") {
			newStatus = "blocked"
		}
		row.Set("status", newStatus)
		if err := app.Save(row); err != nil {
			return fmt.Errorf("M22b migration: backfill row %s: %w", row.Id, err)
		}
		if newStatus == "blocked" {
			if err := audit.WriteRef(app, nil, audit.ActionBlock, "gamertags", row.Id, audit.BlockPayload{
				Reason: "migrated from M7 blocked:bool",
			}); err != nil {
				return fmt.Errorf("M22b migration: synthetic audit for %s: %w", row.Id, err)
			}
		}
	}

	if hasBlocked {
		col, err = app.FindCollectionByNameOrId("gamertags")
		if err != nil {
			return err
		}
		if blockedField := col.Fields.GetByName("blocked"); blockedField != nil {
			col.Fields.RemoveById(blockedField.GetId())
			if err := app.Save(col); err != nil {
				return fmt.Errorf("M22b migration: drop blocked field: %w", err)
			}
		}
	}

	return nil
}

func reconcileGamertagsRules(app *pocketbase.PocketBase) error {
	existing, err := app.FindCollectionByNameOrId("gamertags")
	if err != nil {
		return err
	}
	setGamertagsRules(existing)
	return app.Save(existing)
}

// setGamertagsRules:
//   - List/View: any authed user can browse tags (display-only).
//   - Create:    owner adding their own, OR admin acting on behalf.
//   - Update:    owner editing their non-blocked tag, OR admin. Note the rule
//     gates *whether* an update can land, not *which fields* can be touched —
//     the gamertags_status_transitions hook enforces "owners can't promote
//     themselves to approved" and the auto-downgrade-on-edit semantic.
//   - Delete:    owner deleting their non-blocked tag, OR admin. Blocked rows
//     stay around so the (user, sanitized) constraint blocks resurrection.
func setGamertagsRules(c *core.Collection) {
	listView := "@request.auth.id != \"\""
	create := "@request.auth.id = user.id || @request.auth.isAdmin = true"
	update := "(@request.auth.id = user.id && status != \"blocked\") || @request.auth.isAdmin = true"
	del := "(@request.auth.id = user.id && status != \"blocked\") || @request.auth.isAdmin = true"

	c.ListRule = &listView
	c.ViewRule = &listView
	c.CreateRule = &create
	c.UpdateRule = &update
	c.DeleteRule = &del
}
