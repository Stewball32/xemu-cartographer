package isoingest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	"github.com/Stewball32/xemu-cartographer/internal/lansync"
)

// testCatalog builds a minimal `isos` collection mirroring the managed ingest
// shape and a Config rooted at temp dirs. extract-xiso is stubbed with `true`
// (exits 0, empty tree) so ingest's async extract is deterministic + offline.
func testCatalog(t *testing.T) (core.App, lansync.Config) {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)

	c := core.NewBaseCollection("isos")
	c.Fields.Add(
		&core.TextField{Name: "name"},
		&core.TextField{Name: "filename"},
		&core.TextField{Name: "content_hash"},
		&core.NumberField{Name: "file_size", OnlyInt: true},
		&core.NumberField{Name: "file_mtime", OnlyInt: true},
		&core.SelectField{Name: "role", Values: []string{"play", "server", "shelved"}, MaxSelect: 1},
		&core.BoolField{Name: "allow_on_xbox"},
		&core.BoolField{Name: "drift_detected"},
		&core.TextField{Name: "extracted_path"},
		&core.BoolField{Name: "extracted_ready"},
		&core.DateField{Name: "extracted_at"},
		&core.NumberField{Name: "footprint_bytes", OnlyInt: true},
	)
	if err := app.Save(c); err != nil {
		t.Fatalf("save isos: %v", err)
	}

	root := t.TempDir()
	cfg := lansync.Config{
		InboxDir:       filepath.Join(root, "inbox"),
		ISODir:         filepath.Join(root, "lib"),
		ExtractDir:     filepath.Join(root, "extract"),
		FATXCluster:    16384,
		ExtractXISOCmd: "true",
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("ensure dirs: %v", err)
	}
	return app, cfg
}

func drop(t *testing.T, cfg lansync.Config, name string, data []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(cfg.InboxDir, name), data, 0o644); err != nil {
		t.Fatalf("drop %s: %v", name, err)
	}
}

// TestIngest_HappyPath: a dropped file becomes a managed <id>.iso, hashed +
// frozen read-only, removed from the inbox, with a row landing shelved (the
// organizer sets role + bindings in the Discs detail).
func TestIngest_HappyPath(t *testing.T) {
	app, cfg := testCatalog(t)
	drop(t, cfg, "Halo CE.iso", []byte("disc-one-bytes"))

	res, err := IngestInbox(app, cfg)
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if len(res.Ingested) != 1 || len(res.Skipped) != 0 || len(res.Errors) != 0 {
		t.Fatalf("expected 1 ingested, got %+v", res)
	}
	item := res.Ingested[0]
	if item.Name != "Halo CE" {
		t.Errorf("display name = %q, want derived 'Halo CE'", item.Name)
	}

	// Managed file exists as <id>.iso, inbox is empty.
	managed := cfg.ManagedISOPath(item.ID)
	fi, err := os.Stat(managed)
	if err != nil {
		t.Fatalf("managed file missing: %v", err)
	}
	if fi.Mode().Perm() != 0o444 {
		t.Errorf("managed file mode = %v, want 0444 (frozen)", fi.Mode().Perm())
	}
	if left, _ := os.ReadDir(cfg.InboxDir); len(left) != 0 {
		t.Errorf("inbox should be empty after ingest, has %d", len(left))
	}

	rec, _ := app.FindRecordById(collectionName, item.ID)
	if rec.GetString("content_hash") == "" {
		t.Error("content_hash not stored")
	}
	if rec.GetString("role") != "shelved" || rec.GetBool("drift_detected") {
		t.Errorf("row should land shelved + undrifted; got role=%q drift=%v",
			rec.GetString("role"), rec.GetBool("drift_detected"))
	}
	if rec.GetInt("file_size") == 0 {
		t.Error("file_size anchor not stored")
	}
}

// TestIngest_DedupeByHash: a second file with identical bytes is skipped as a
// duplicate (no second row).
func TestIngest_DedupeByHash(t *testing.T) {
	app, cfg := testCatalog(t)
	drop(t, cfg, "original.iso", []byte("same-bytes"))
	if _, err := IngestInbox(app, cfg); err != nil {
		t.Fatalf("ingest 1: %v", err)
	}
	drop(t, cfg, "copy.iso", []byte("same-bytes"))
	res, err := IngestInbox(app, cfg)
	if err != nil {
		t.Fatalf("ingest 2: %v", err)
	}
	if len(res.Ingested) != 0 || len(res.Skipped) != 1 {
		t.Fatalf("expected duplicate skip, got %+v", res)
	}
	if res.Skipped[0].DupOf != "original" {
		t.Errorf("dup_of = %q, want 'original'", res.Skipped[0].DupOf)
	}
	rows, _ := app.FindAllRecords(collectionName)
	if len(rows) != 1 {
		t.Errorf("catalog should hold 1 row, has %d", len(rows))
	}
}

// TestDrift_Detected: tampering with the managed bytes trips VerifyAndFlag,
// which forces the row to shelved + flagged (losing its play role).
func TestDrift_Detected(t *testing.T) {
	app, cfg := testCatalog(t)
	drop(t, cfg, "game.iso", []byte("pristine-bytes"))
	res, err := IngestInbox(app, cfg)
	if err != nil || len(res.Ingested) != 1 {
		t.Fatalf("ingest: %v %+v", err, res)
	}
	id := res.Ingested[0].ID
	managed := cfg.ManagedISOPath(id)

	rec, _ := app.FindRecordById(collectionName, id)
	if ok, _ := VerifyManaged(cfg, rec); !ok {
		t.Fatal("pristine disc should verify OK")
	}
	// Promote to play so the drift demotion below is observable.
	rec.Set("role", "play")
	if err := app.Save(rec); err != nil {
		t.Fatalf("promote: %v", err)
	}

	// Tamper: rewrite the managed bytes (different size → cheap check re-hashes).
	_ = os.Chmod(managed, 0o644)
	if err := os.WriteFile(managed, []byte("tampered-different-length-bytes"), 0o644); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	rec, _ = app.FindRecordById(collectionName, id)
	if ok, reason := VerifyManaged(cfg, rec); ok || reason == "" {
		t.Fatalf("tampered disc should fail verify; got ok=%v reason=%q", ok, reason)
	}
	if VerifyAndFlag(app, cfg, rec) {
		t.Fatal("VerifyAndFlag should report bad bytes")
	}
	reloaded, _ := app.FindRecordById(collectionName, id)
	if reloaded.GetString("role") != "shelved" || !reloaded.GetBool("drift_detected") {
		t.Errorf("drift row should be shelved + flagged; got role=%q drift=%v",
			reloaded.GetString("role"), reloaded.GetBool("drift_detected"))
	}
}
