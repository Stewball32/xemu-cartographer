package isoingest

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestGameForTitleID(t *testing.T) {
	cases := map[string]string{
		"4D530004": "ce",
		"4D530064": "h2",
		"41560017": "", // some other title
		"":         "",
	}
	for in, want := range cases {
		if got := GameForTitleID(in); got != want {
			t.Errorf("GameForTitleID(%q) = %q, want %q", in, got, want)
		}
	}
}

// catalogTestApp builds isos + iso_maps + maps collections (minimal shapes).
func catalogTestApp(t *testing.T) core.App {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)

	isos := core.NewBaseCollection("isos")
	isos.Fields.Add(
		&core.TextField{Name: "name"},
		&core.TextField{Name: "title_id"},
		&core.TextField{Name: "extracted_path"},
		&core.BoolField{Name: "extracted_ready"},
	)
	if err := app.Save(isos); err != nil {
		t.Fatalf("save isos: %v", err)
	}

	isoMaps := core.NewBaseCollection("iso_maps")
	isoMaps.Fields.Add(
		&core.RelationField{Name: "iso", CollectionId: isos.Id, MaxSelect: 1},
		&core.TextField{Name: "filename"},
		&core.TextField{Name: "name"},
		&core.TextField{Name: "map_type"},
		&core.TextField{Name: "content_hash"},
	)
	if err := app.Save(isoMaps); err != nil {
		t.Fatalf("save iso_maps: %v", err)
	}

	maps := core.NewBaseCollection("maps")
	maps.Fields.Add(
		&core.TextField{Name: "filename"},
		&core.TextField{Name: "content_hash"},
		&core.SelectField{Name: "game", Values: []string{"ce", "h2"}, MaxSelect: 1},
		&core.TextField{Name: "display_name"},
	)
	if err := app.Save(maps); err != nil {
		t.Fatalf("save maps: %v", err)
	}
	return app
}

// TestSyncCatalog: multiplayer hashed rows mint catalog builds; identical
// (filename, hash) across discs collapse; campaign/unhashed rows are skipped;
// existing curated rows are never re-minted or touched.
func TestSyncCatalog(t *testing.T) {
	app := catalogTestApp(t)
	isosCol, _ := app.FindCollectionByNameOrId("isos")
	mapsCol, _ := app.FindCollectionByNameOrId("iso_maps")

	newISO := func(name, titleID string) *core.Record {
		r := core.NewRecord(isosCol)
		r.Set("name", name)
		r.Set("title_id", titleID)
		if err := app.Save(r); err != nil {
			t.Fatalf("save iso: %v", err)
		}
		return r
	}
	newRow := func(isoID, filename, mapType, hash string) {
		r := core.NewRecord(mapsCol)
		r.Set("iso", isoID)
		r.Set("filename", filename)
		r.Set("map_type", mapType)
		r.Set("content_hash", hash)
		if err := app.Save(r); err != nil {
			t.Fatalf("save iso_map: %v", err)
		}
	}

	stock := newISO("Halo CE (stock)", "4D530004")
	nhe := newISO("Halo CE (NHE)", "4D530004")
	unknown := newISO("Mystery disc", "12345678")

	newRow(stock.Id, "bloodgulch.map", "multiplayer", "aaa")
	newRow(stock.Id, "a10.map", "campaign", "ccc")         // campaign — skipped
	newRow(stock.Id, "ui.map", "ui", "")                   // unhashed — skipped
	newRow(nhe.Id, "bloodgulch.map", "multiplayer", "aaa") // same build — collapses
	newRow(nhe.Id, "damnation.map", "multiplayer", "bbb")  // NHE retune — own row
	newRow(unknown.Id, "custom.map", "multiplayer", "ddd") // unknown game — skipped

	SyncCatalog(app, stock.Id)
	SyncCatalog(app, nhe.Id)
	SyncCatalog(app, unknown.Id)

	rows, err := app.FindAllRecords("maps")
	if err != nil {
		t.Fatalf("list catalog: %v", err)
	}
	if len(rows) != 2 {
		for _, r := range rows {
			t.Logf("row: %s %s %s", r.GetString("game"), r.GetString("filename"), r.GetString("content_hash"))
		}
		t.Fatalf("catalog rows = %d, want 2 (bloodgulch@aaa collapsed, damnation@bbb)", len(rows))
	}

	// Curate one, re-sync, and prove the edit survives (create-only).
	var bg *core.Record
	for _, r := range rows {
		if r.GetString("content_hash") == "aaa" {
			bg = r
		}
	}
	bg.Set("display_name", "Blood Gulch")
	if err := app.Save(bg); err != nil {
		t.Fatalf("curate: %v", err)
	}
	SyncCatalog(app, stock.Id)
	reloaded, _ := app.FindRecordById("maps", bg.Id)
	if reloaded.GetString("display_name") != "Blood Gulch" {
		t.Error("re-sync clobbered organizer curation")
	}
	if rows, _ := app.FindAllRecords("maps"); len(rows) != 2 {
		t.Errorf("re-sync minted duplicates: %d rows", len(rows))
	}
}
