package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerAuditLogCollection)
}

// registerAuditLogCollection creates the audit_log collection per ADR-0002.
// One row per state-changing admin/system event across every collection that
// needs traceable history — gamertag moderation (M22b), team-name moderation
// (M22c), team renames (M22d), role grants (M8), game corrections (M13+),
// etc. The same shape covers all of them so cross-collection queries (e.g.
// "everything this admin did in the last 24h") are single-table reads.
//
// actor is nullable so system-triggered events (cron jobs, migrations,
// backfills) record as actor=null without a synthetic "system" user.
//
// target_id is text rather than a relation so audit rows survive hard-delete
// of the target — a deleted user's record of bans still resolves. The cost
// is that PB's expand mechanic doesn't apply; consumers do explicit lookups.
//
// payload_json is opaque to the DB. The internal/audit package owns the
// per-action shape via typed payload structs and the Write/WriteRef helpers;
// direct PB SDK writes from outside that package are an anti-pattern (a
// payload typo at write time silently corrupts the row).
//
// PB rules: list/view admin-only because the trail exposes mod actions and
// actors. Create/update/delete are nil — every legitimate write goes through
// the server-side helper. The isAdmin gate matches the rest of the M7-era
// schemas and will swap to role-based checks when M8 lands.
func registerAuditLogCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "audit_log") {
		return reconcileAuditLogRules(app)
	}

	collection := core.NewBaseCollection("audit_log")

	usersCol, err := requireCollection(app, "users")
	if err != nil {
		return err
	}

	collection.Fields.Add(
		&core.RelationField{
			Name:          "actor",
			CollectionId:  usersCol.Id,
			MaxSelect:     1,
			CascadeDelete: false,
		},
		&core.TextField{
			Name:     "target_collection",
			Required: true,
			Min:      1,
		},
		&core.TextField{
			Name:     "target_id",
			Required: true,
			Min:      1,
		},
		&core.TextField{
			Name:     "action",
			Required: true,
			Min:      1,
		},
		&core.JSONField{
			Name:    "payload_json",
			MaxSize: 1 << 19, // 512 KiB — payloads are small structs; cap keeps a single bad write from blowing the row budget
		},
		&core.AutodateField{
			Name:     "created",
			OnCreate: true,
		},
	)

	// Indexes per ADR-0002. Each covers one of the three documented read
	// patterns; created DESC suffix puts newest rows first so LIMIT 1 reads
	// the latest entry without an extra ORDER BY scan.
	collection.AddIndex("idx_audit_log_target_created", false, "target_collection, target_id, created DESC", "")
	collection.AddIndex("idx_audit_log_actor_created", false, "actor, created DESC", "")
	collection.AddIndex("idx_audit_log_action_created", false, "action, created DESC", "")

	setAuditLogRules(collection)

	return app.Save(collection)
}

func reconcileAuditLogRules(app *pocketbase.PocketBase) error {
	existing, err := app.FindCollectionByNameOrId("audit_log")
	if err != nil {
		return err
	}
	setAuditLogRules(existing)
	return app.Save(existing)
}

func setAuditLogRules(c *core.Collection) {
	adminOnly := "@request.auth.isAdmin = true"
	c.ListRule = &adminOnly
	c.ViewRule = &adminOnly
	c.CreateRule = nil
	c.UpdateRule = nil
	c.DeleteRule = nil
}
