package audit_test

import (
	"encoding/json"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	"github.com/Stewball32/xemu-cartographer/internal/audit"
)

// ensureAuditLogCollection registers the audit_log collection on a fresh
// TestApp. Mirrors schema/audit_log.go's shape; kept inline so the test
// package doesn't have to import the schema package (which would trigger
// every M7 collection's init() and couple the test to that ordering).
func ensureAuditLogCollection(t *testing.T, app core.App) {
	t.Helper()

	if _, err := app.FindCollectionByNameOrId("audit_log"); err == nil {
		return
	}

	usersCol, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatalf("lookup users collection: %v", err)
	}

	c := core.NewBaseCollection("audit_log")
	c.Fields.Add(
		&core.RelationField{Name: "actor", CollectionId: usersCol.Id, MaxSelect: 1},
		&core.TextField{Name: "target_collection", Required: true, Min: 1},
		&core.TextField{Name: "target_id", Required: true, Min: 1},
		&core.TextField{Name: "action", Required: true, Min: 1},
		&core.JSONField{Name: "payload_json", MaxSize: 1 << 19},
		&core.AutodateField{Name: "created", OnCreate: true},
	)
	if err := app.Save(c); err != nil {
		t.Fatalf("save audit_log collection: %v", err)
	}
}

type blockPayload struct {
	Reason string `json:"reason"`
}

func TestWriteRefMinimal(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureAuditLogCollection(t, app)

	if err := audit.WriteRef(app, nil, audit.Action("block"), "gamertags", "fake_target_id", blockPayload{Reason: "spam"}); err != nil {
		t.Fatalf("WriteRef: %v", err)
	}

	rows, err := app.FindAllRecords("audit_log")
	if err != nil {
		t.Fatalf("FindAllRecords: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 audit row, got %d", len(rows))
	}
	row := rows[0]

	if got := row.GetString("action"); got != "block" {
		t.Errorf("action = %q, want %q", got, "block")
	}
	if got := row.GetString("target_collection"); got != "gamertags" {
		t.Errorf("target_collection = %q, want %q", got, "gamertags")
	}
	if got := row.GetString("target_id"); got != "fake_target_id" {
		t.Errorf("target_id = %q, want %q", got, "fake_target_id")
	}
	if got := row.GetString("actor"); got != "" {
		t.Errorf("actor = %q, want empty for nil actor", got)
	}

	var payload blockPayload
	if err := json.Unmarshal([]byte(row.GetString("payload_json")), &payload); err != nil {
		t.Fatalf("unmarshal payload_json: %v", err)
	}
	if payload.Reason != "spam" {
		t.Errorf("payload.Reason = %q, want %q", payload.Reason, "spam")
	}
}

func TestWriteFromRecord(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureAuditLogCollection(t, app)

	// Pull a real user record out of the test fixtures to use as both actor
	// and target — exercises target.Collection().Name + target.Id derivation.
	users, err := app.FindAllRecords("users")
	if err != nil {
		t.Fatalf("FindAllRecords users: %v", err)
	}
	if len(users) < 2 {
		t.Fatalf("test fixture must have >= 2 users, got %d", len(users))
	}
	actor, target := users[0], users[1]

	if err := audit.Write(app, actor, audit.Action("ban"), target, nil); err != nil {
		t.Fatalf("Write: %v", err)
	}

	rows, err := app.FindAllRecords("audit_log")
	if err != nil {
		t.Fatalf("FindAllRecords: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 audit row, got %d", len(rows))
	}
	row := rows[0]

	if got := row.GetString("actor"); got != actor.Id {
		t.Errorf("actor = %q, want %q", got, actor.Id)
	}
	if got := row.GetString("target_collection"); got != "users" {
		t.Errorf("target_collection = %q, want %q", got, "users")
	}
	if got := row.GetString("target_id"); got != target.Id {
		t.Errorf("target_id = %q, want %q", got, target.Id)
	}
	// PB serializes an unset JSON column as the literal "null", which is the
	// correct JSON encoding of "no payload". Either "" or "null" is acceptable
	// — both round-trip to a nil Go value on unmarshal.
	if got := row.GetString("payload_json"); got != "" && got != "null" {
		t.Errorf("payload_json = %q, want empty or \"null\" for nil payload", got)
	}
}

func TestValidation(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureAuditLogCollection(t, app)

	if err := audit.Write(app, nil, audit.Action("block"), nil, nil); err == nil {
		t.Errorf("Write with nil target: want error, got nil")
	}
	if err := audit.WriteRef(app, nil, audit.Action(""), "gamertags", "id", nil); err == nil {
		t.Errorf("WriteRef with empty action: want error, got nil")
	}
	if err := audit.WriteRef(app, nil, audit.Action("block"), "", "id", nil); err == nil {
		t.Errorf("WriteRef with empty collection: want error, got nil")
	}
	if err := audit.WriteRef(app, nil, audit.Action("block"), "gamertags", "", nil); err == nil {
		t.Errorf("WriteRef with empty id: want error, got nil")
	}
}
