package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerNotificationsCollection)
}

// registerNotificationsCollection creates the notifications collection landed
// in M23a — the delivery surface that lets one user act on another's identity
// (invites, join requests, accept/decline confirmations, removals). The bell
// icon in the header polls this; the full list at /notifications/ paginates
// over it.
//
// Shape mirrors the audit_log model from ADR-0002 — opaque payload_json with
// per-type structs declared in internal/notifications/. The big difference:
// notifications are addressed to a specific recipient (user FK), where
// audit_log rows are addressed to nobody.
//
// payload_json carries the per-type shape (TeamInvitePayload, etc.). The
// internal/notifications package owns the write path; direct PB SDK writes
// from outside that package are an anti-pattern — same rationale as audit.
//
// PB rules: list/view = own or admin; update = own or admin (the
// notifications_field_lock hook enforces non-admin updates only touch `read`
// + `read_at`); create/delete = admin-only because legitimate writes go
// through the server-side helper.
func registerNotificationsCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "notifications") {
		return reconcileNotificationsRules(app)
	}

	collection := core.NewBaseCollection("notifications")

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
			Name:     "type",
			Required: true,
			Min:      1,
		},
		&core.JSONField{
			Name:    "payload_json",
			MaxSize: 1 << 18, // 256 KiB — payloads are small UI structs.
		},
		&core.BoolField{Name: "read"},
		&core.DateField{Name: "read_at"},
		&core.AutodateField{
			Name:     "created",
			OnCreate: true,
		},
	)

	// Bell-count walk: filter `user = X AND read = false`, no sort needed.
	// Full-page walk: filter `user = X`, sort `-created`. Both hit the same
	// covering index.
	collection.AddIndex("idx_notifications_user_read_created", false, "user, read, created DESC", "")
	collection.AddIndex("idx_notifications_user_created", false, "user, created DESC", "")

	setNotificationsRules(collection)

	return app.Save(collection)
}

func reconcileNotificationsRules(app *pocketbase.PocketBase) error {
	existing, err := app.FindCollectionByNameOrId("notifications")
	if err != nil {
		return err
	}
	setNotificationsRules(existing)
	return app.Save(existing)
}

// setNotificationsRules:
//   - List/View: recipient OR admin. The PB rule has to allow the owner to
//     see their own rows so the bell + /notifications/ page can read them.
//   - Update:    recipient OR admin. The notifications_field_lock hook
//     restricts non-admin updates to the `read` + `read_at` fields so an
//     owner can mark-as-read but can't tamper with type/payload.
//   - Create/Delete: nil — clients can't fabricate notifications, only the
//     server-side notifications.Notify helper does (which bypasses rules via
//     app.Save). Admin can still drop rows through the admin SDK if needed.
func setNotificationsRules(c *core.Collection) {
	own := "user = @request.auth.id || @request.auth.isAdmin = true"

	c.ListRule = &own
	c.ViewRule = &own
	c.CreateRule = nil
	c.UpdateRule = &own
	c.DeleteRule = nil
}
