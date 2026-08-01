package isos

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// TestValidFilename covers the bare-filename guard shared by create + update:
// the catalog may only reference a plain filename inside the shared ISO library,
// never a path that could escape it.
func TestValidFilename(t *testing.T) {
	cases := map[string]bool{
		"halo-ce.iso":   true,
		"Halo 2.iso":    true,
		"":              false,
		"..":            false,
		".hidden.iso":   false,
		"sub/halo.iso":  false,
		"a\\b.iso":      false,
		"../escape.iso": false,
		"/abs/path.iso": false,
	}
	for in, want := range cases {
		if got := validFilename(in); got != want {
			t.Errorf("validFilename(%q) = %v, want %v", in, got, want)
		}
	}
}

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
