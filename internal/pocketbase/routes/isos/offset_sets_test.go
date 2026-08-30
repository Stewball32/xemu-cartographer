package isos

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

// offsetSetsTestApp builds minimal isos + offset_sets collections.
func offsetSetsTestApp(t *testing.T) core.App {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)

	isos := core.NewBaseCollection(collectionName)
	isos.Fields.Add(
		&core.TextField{Name: "name"},
		&core.TextField{Name: "offset_set"},
	)
	if err := app.Save(isos); err != nil {
		t.Fatalf("save isos: %v", err)
	}

	sets := core.NewBaseCollection(offsetSetsCollection)
	sets.Fields.Add(
		&core.TextField{Name: "set_id"},
		&core.TextField{Name: "game"},
		&core.TextField{Name: "description"},
		&core.FileField{Name: "file", MaxSelect: 1, MaxSize: 5 << 20},
		&core.NumberField{Name: "count", OnlyInt: true},
		&core.NumberField{Name: "version", OnlyInt: true},
		&core.TextField{Name: "source_name"},
	)
	if err := app.Save(sets); err != nil {
		t.Fatalf("save offset_sets: %v", err)
	}
	return app
}

// TestBoundCounts: counts group by offset_set id, unbound rows don't count.
func TestBoundCounts(t *testing.T) {
	app := offsetSetsTestApp(t)
	col, _ := app.FindCollectionByNameOrId(collectionName)
	for _, set := range []string{"nhe-1.1", "nhe-1.1", "ce-baseline", ""} {
		r := core.NewRecord(col)
		r.Set("name", "disc")
		r.Set("offset_set", set)
		if err := app.Save(r); err != nil {
			t.Fatalf("save disc: %v", err)
		}
	}
	counts := boundCounts(app)
	if counts["nhe-1.1"] != 2 || counts["ce-baseline"] != 1 || counts[""] != 0 {
		t.Errorf("boundCounts = %v", counts)
	}
}

// TestOffsetSetRaw: embedded ids serve their compiled-in bytes; a stored import
// serves its upload byte-identical with its original filename; unknown ids
// error.
func TestOffsetSetRaw(t *testing.T) {
	app := offsetSetsTestApp(t)

	raw, name, err := offsetSetRaw(app, "ce-baseline")
	if err != nil || len(raw) == 0 || name != "ce-baseline.json" {
		t.Fatalf("embedded: (%d bytes, %q, %v)", len(raw), name, err)
	}

	upload := []byte(`{"game":"haloce","id":"nhe-1.2","offsets":{"X":{"value":"0x1","type":"address"}}}`)
	f, err := filesystem.NewFileFromBytes(upload, "nhe-1.2.offsetmap.json")
	if err != nil {
		t.Fatalf("file: %v", err)
	}
	col, _ := app.FindCollectionByNameOrId(offsetSetsCollection)
	rec := core.NewRecord(col)
	rec.Set("set_id", "nhe-1.2")
	rec.Set("game", "haloce")
	rec.Set("source_name", "nhe-1.2.offsetmap.json")
	rec.Set("file", f)
	if err := app.Save(rec); err != nil {
		t.Fatalf("save set: %v", err)
	}

	got, gotName, err := offsetSetRaw(app, "nhe-1.2")
	if err != nil {
		t.Fatalf("stored: %v", err)
	}
	if string(got) != string(upload) {
		t.Error("stored bytes not byte-identical to the upload")
	}
	if gotName != "nhe-1.2.offsetmap.json" {
		t.Errorf("source name = %q", gotName)
	}

	if _, _, err := offsetSetRaw(app, "no-such-set"); err == nil {
		t.Error("unknown id should error")
	}
}

// TestOffsetSetExists: embedded and imported ids both count; unknown ids don't.
func TestOffsetSetExists(t *testing.T) {
	app := offsetSetsTestApp(t)
	col, _ := app.FindCollectionByNameOrId(offsetSetsCollection)
	rec := core.NewRecord(col)
	rec.Set("set_id", "nhe-9.9")
	rec.Set("game", "haloce")
	if err := app.Save(rec); err != nil {
		t.Fatalf("save: %v", err)
	}
	for id, want := range map[string]bool{"ce-baseline": true, "nhe-9.9": true, "nope": false} {
		if got := offsetSetExists(app, id); got != want {
			t.Errorf("offsetSetExists(%q) = %v, want %v", id, got, want)
		}
	}
}
