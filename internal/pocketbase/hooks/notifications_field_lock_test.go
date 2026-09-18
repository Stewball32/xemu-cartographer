package hooks

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"

	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/pb/pbtest"
)

// notificationsApp is pbtest.NewApp plus a notifications collection shaped
// like schema/notifications.go (the pbtest schema does not carry it).
func notificationsApp(t *testing.T) core.App {
	t.Helper()
	app, _ := pbtest.NewApp(t)
	if _, err := app.FindCollectionByNameOrId("notifications"); err == nil {
		return app
	}
	usersCol, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatalf("users collection: %v", err)
	}
	c := core.NewBaseCollection("notifications")
	c.Fields.Add(
		&core.RelationField{Name: "user", CollectionId: usersCol.Id, MaxSelect: 1, Required: true},
		&core.TextField{Name: "type", Required: true, Min: 1},
		&core.JSONField{Name: "payload_json", MaxSize: 1 << 18},
		&core.BoolField{Name: "read"},
		&core.DateField{Name: "read_at"},
		&core.AutodateField{Name: "created", OnCreate: true},
	)
	if err := app.Save(c); err != nil {
		t.Fatalf("save notifications collection: %v", err)
	}
	return app
}

// newNotification creates an unread notification row for userID.
func newNotification(t *testing.T, app core.App, userID string) *core.Record {
	t.Helper()
	col, err := app.FindCollectionByNameOrId("notifications")
	if err != nil {
		t.Fatalf("notifications collection: %v", err)
	}
	rec := core.NewRecord(col)
	rec.Set("user", userID)
	rec.Set("type", "team_invite")
	rec.Set("payload_json", `{"team_id":"t1"}`)
	if err := app.Save(rec); err != nil {
		t.Fatalf("save notification: %v", err)
	}
	return reload(t, app, rec)
}

func TestNotificationsFieldLock_InternalActorAllowed(t *testing.T) {
	app := notificationsApp(t)
	recipient := pbtest.NewUser(t, app, "recipient@test.dev")
	n := newNotification(t, app, recipient.Id)

	// The in-process writer may correct any field.
	n.Set("type", "team_kick")
	n.Set("payload_json", `{"team_id":"t2"}`)
	if err := notificationsFieldLock(authzRequestEvent(app, nil, n)); err != nil {
		t.Fatalf("internal actor: %v", err)
	}
}

func TestNotificationsFieldLock_MemberDenied(t *testing.T) {
	app := notificationsApp(t)
	recipient := memberUser(t, app, "recipient@test.dev")
	n := newNotification(t, app, recipient.Id)

	// The recipient may only flip read (+ derived read_at)…
	n.Set("read", true)
	if err := notificationsFieldLock(authzRequestEvent(app, recipient, n)); err != nil {
		t.Fatalf("recipient marking read: %v", err)
	}
	if n.GetDateTime("read_at").IsZero() {
		t.Fatal("read_at not stamped on read=true")
	}

	// …and every other field stays locked.
	for field, value := range map[string]any{
		"type":         "team_kick",
		"payload_json": `{"team_id":"t2"}`,
		"user":         pbtest.NewUser(t, app, "other@test.dev").Id,
	} {
		tamper := reload(t, app, n)
		tamper.Set(field, value)
		wantAPIError(t, notificationsFieldLock(authzRequestEvent(app, recipient, tamper)), 400)
	}
}

func TestNotificationsFieldLock_AdminAllowed(t *testing.T) {
	app := notificationsApp(t)
	admin := adminUser(t, app, "admin@test.dev")
	recipient := pbtest.NewUser(t, app, "recipient@test.dev")
	n := newNotification(t, app, recipient.Id)

	n.Set("type", "team_kick")
	if err := notificationsFieldLock(authzRequestEvent(app, admin, n)); err != nil {
		t.Fatalf("admin: %v", err)
	}
}
