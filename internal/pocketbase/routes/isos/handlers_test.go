package isos

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// TestResolveServerISO covers the optional server_iso validation: "" clears the
// link, a self-reference is rejected, a non-existent id is rejected, and an
// existing catalog id is accepted.
func TestResolveServerISO(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)

	col := core.NewBaseCollection(collectionName)
	col.Fields.Add(&core.TextField{Name: "filename"})
	if err := app.Save(col); err != nil {
		t.Fatalf("save isos: %v", err)
	}
	existing := core.NewRecord(col)
	existing.Set("filename", "server.iso")
	if err := app.Save(existing); err != nil {
		t.Fatalf("save existing: %v", err)
	}

	t.Run("empty clears the link", func(t *testing.T) {
		id, msg := resolveServerISO(app, "   ", "self-id")
		if id != "" || msg != "" {
			t.Fatalf("got (%q,%q), want empty clear", id, msg)
		}
	})
	t.Run("self-reference rejected", func(t *testing.T) {
		if _, msg := resolveServerISO(app, existing.Id, existing.Id); msg == "" {
			t.Error("expected rejection for self-reference")
		}
	})
	t.Run("non-existent rejected", func(t *testing.T) {
		if _, msg := resolveServerISO(app, "no-such-id", "self-id"); msg == "" {
			t.Error("expected rejection for non-existent id")
		}
	})
	t.Run("existing accepted", func(t *testing.T) {
		id, msg := resolveServerISO(app, existing.Id, "self-id")
		if id != existing.Id || msg != "" {
			t.Fatalf("got (%q,%q), want (%q,\"\")", id, msg, existing.Id)
		}
	})
}
