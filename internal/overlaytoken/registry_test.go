package overlaytoken

import (
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func ensureCollections(t *testing.T, app core.App) {
	t.Helper()
	usersCol, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatalf("users collection: %v", err)
	}

	if _, err := app.FindCollectionByNameOrId("audit_log"); err != nil {
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
			t.Fatalf("save audit_log: %v", err)
		}
	}

	if _, err := app.FindCollectionByNameOrId("overlay_tokens"); err != nil {
		c := core.NewBaseCollection("overlay_tokens")
		c.Fields.Add(
			&core.TextField{Name: "kid", Required: true},
			&core.TextField{Name: "room", Required: true},
			&core.TextField{Name: "scope"},
			&core.TextField{Name: "label"},
			&core.RelationField{Name: "minted_by", CollectionId: usersCol.Id, MaxSelect: 1},
			&core.DateField{Name: "expires_at"},
			&core.BoolField{Name: "revoked"},
			&core.DateField{Name: "revoked_at"},
			&core.RelationField{Name: "revoked_by", CollectionId: usersCol.Id, MaxSelect: 1},
			&core.AutodateField{Name: "created", OnCreate: true},
		)
		c.AddIndex("idx_overlay_tokens_kid_unique", true, "kid", "")
		if err := app.Save(c); err != nil {
			t.Fatalf("save overlay_tokens: %v", err)
		}
	}
}

func auditCount(t *testing.T, app core.App, action string) int {
	t.Helper()
	rows, err := app.FindAllRecords("audit_log")
	if err != nil {
		t.Fatalf("FindAllRecords audit_log: %v", err)
	}
	n := 0
	for _, r := range rows {
		if r.GetString("action") == action {
			n++
		}
	}
	return n
}

func TestMint_Active_Revoke(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureCollections(t, app)
	Configure(testSecret)

	now := time.Now()
	m, err := Mint(app, "host:pod-a", "OBS scoreboard", time.Hour, nil, now)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if m.Token == "" || m.Kid == "" || m.Room != "host:pod-a" {
		t.Fatalf("unexpected Minted: %+v", m)
	}

	// Handshake-style verification of the live token.
	if room, ok := VerifyActive(app, m.Token, now); !ok || room != "host:pod-a" {
		t.Errorf("VerifyActive(valid) = (%q, %v), want (host:pod-a, true)", room, ok)
	}
	// Registry checks.
	if !Active(app, m.Kid, "host:pod-a", now) {
		t.Error("freshly minted token should be Active")
	}
	if Active(app, m.Kid, "host:pod-b", now) {
		t.Error("token must not be Active for a different room")
	}
	if n := auditCount(t, app, "overlay_mint"); n != 1 {
		t.Errorf("overlay_mint audit rows = %d, want 1", n)
	}

	// Revoke takes effect immediately.
	if err := Revoke(app, m.Kid, nil); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if Active(app, m.Kid, "host:pod-a", now) {
		t.Error("revoked token should not be Active")
	}
	if room, ok := VerifyActive(app, m.Token, now); ok {
		t.Errorf("VerifyActive after revoke = (%q, %v), want not ok", room, ok)
	}
	if n := auditCount(t, app, "overlay_revoke"); n != 1 {
		t.Errorf("overlay_revoke audit rows = %d, want 1", n)
	}

	// Revoking again is a harmless no-op (no second audit row).
	if err := Revoke(app, m.Kid, nil); err != nil {
		t.Fatalf("second Revoke: %v", err)
	}
	if n := auditCount(t, app, "overlay_revoke"); n != 1 {
		t.Errorf("re-revoke wrote an extra audit row: %d, want 1", n)
	}
}

func TestActive_Expired(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureCollections(t, app)
	Configure(testSecret)

	past := time.Now().Add(-2 * time.Hour)
	m, err := Mint(app, "host:pod-a", "", time.Hour, nil, past) // expired 1h ago
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if Active(app, m.Kid, "host:pod-a", time.Now()) {
		t.Error("expired token should not be Active")
	}
	if _, ok := VerifyActive(app, m.Token, time.Now()); ok {
		t.Error("VerifyActive should reject an expired token")
	}
}

func TestActive_UnknownKid(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureCollections(t, app)
	if Active(app, "no-such-kid", "host:pod-a", time.Now()) {
		t.Error("unknown kid should not be Active")
	}
}
