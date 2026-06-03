package notifications_test

import (
	"encoding/json"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	"github.com/Stewball32/xemu-cartographer/internal/notifications"
)

// ensureNotificationsCollection registers the notifications collection on a
// fresh TestApp. Mirrors schema/notifications.go's shape; kept inline so
// the test package doesn't have to import the schema package (which would
// trigger every M7 collection's init() and couple the test to that
// ordering — same pattern as audit's ensureAuditLogCollection).
func ensureNotificationsCollection(t *testing.T, app core.App) {
	t.Helper()

	if _, err := app.FindCollectionByNameOrId("notifications"); err == nil {
		return
	}

	usersCol, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatalf("lookup users collection: %v", err)
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
}

type fakePayload struct {
	TeamID string `json:"team_id"`
}

func TestNotifyMinimal(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureNotificationsCollection(t, app)

	users, err := app.FindAllRecords("users")
	if err != nil {
		t.Fatalf("FindAllRecords users: %v", err)
	}
	if len(users) == 0 {
		t.Fatalf("test fixture must have >= 1 user")
	}
	user := users[0]

	if err := notifications.Notify(app, user, notifications.Type("team_invite"), fakePayload{TeamID: "abc123"}); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	rows, err := app.FindAllRecords("notifications")
	if err != nil {
		t.Fatalf("FindAllRecords: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 notifications row, got %d", len(rows))
	}
	row := rows[0]

	if got := row.GetString("user"); got != user.Id {
		t.Errorf("user = %q, want %q", got, user.Id)
	}
	if got := row.GetString("type"); got != "team_invite" {
		t.Errorf("type = %q, want %q", got, "team_invite")
	}
	if got := row.GetBool("read"); got {
		t.Errorf("read = true, want false on a fresh write")
	}

	var payload fakePayload
	if err := json.Unmarshal([]byte(row.GetString("payload_json")), &payload); err != nil {
		t.Fatalf("unmarshal payload_json: %v", err)
	}
	if payload.TeamID != "abc123" {
		t.Errorf("payload.TeamID = %q, want %q", payload.TeamID, "abc123")
	}
}

func TestNotifyNilPayload(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureNotificationsCollection(t, app)

	users, err := app.FindAllRecords("users")
	if err != nil {
		t.Fatalf("FindAllRecords users: %v", err)
	}
	user := users[0]

	if err := notifications.Notify(app, user, notifications.Type("team_invite"), nil); err != nil {
		t.Fatalf("Notify with nil payload: %v", err)
	}

	rows, err := app.FindAllRecords("notifications")
	if err != nil {
		t.Fatalf("FindAllRecords: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 notifications row, got %d", len(rows))
	}
	// PB serializes an unset JSON column as "" or "null" — both round-trip
	// to a nil Go value on unmarshal.
	if got := rows[0].GetString("payload_json"); got != "" && got != "null" {
		t.Errorf("payload_json = %q, want empty or \"null\" for nil payload", got)
	}
}

func TestNotifyValidation(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureNotificationsCollection(t, app)

	if err := notifications.Notify(app, nil, notifications.Type("team_invite"), nil); err == nil {
		t.Errorf("Notify with nil user: want error, got nil")
	}

	users, err := app.FindAllRecords("users")
	if err != nil {
		t.Fatalf("FindAllRecords users: %v", err)
	}
	user := users[0]
	if err := notifications.Notify(app, user, notifications.Type(""), nil); err == nil {
		t.Errorf("Notify with empty type: want error, got nil")
	}
}
