package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Map-list + map-thumbnail extraction (the map-graphics feature). Each ingested
// build's extracted maps/*.map become rows in `iso_maps` — one per cache with
// its internal name + type, plus a `thumb` image field carrying the top-down
// BSP render for multiplayer maps (CE has no embedded preview art, so the
// render IS the graphic). Rows cascade-delete with their parent isos row.
//
// Server-managed: written only by the ingest pipeline (app.Save bypasses rules);
// read access is any authed user so the organizer + play surfaces can list maps
// and load the (non-protected) thumbnails. Additive + reversible.
func init() {
	m.Register(func(app core.App) error {
		if _, err := app.FindCollectionByNameOrId("iso_maps"); err == nil {
			return nil // idempotent
		}
		isos, err := app.FindCollectionByNameOrId("isos")
		if err != nil {
			return err
		}
		c := core.NewBaseCollection("iso_maps")
		c.Fields.Add(
			&core.RelationField{
				Name:          "iso",
				CollectionId:  isos.Id,
				CascadeDelete: true, // deleting a build removes its map rows + thumbs
				MaxSelect:     1,
				Required:      true,
			},
			&core.TextField{Name: "filename", Required: true, Max: 128},
			&core.TextField{Name: "name", Max: 128},
			&core.TextField{Name: "map_type", Max: 32},
			&core.FileField{
				Name:      "thumb",
				MaxSelect: 1,
				MaxSize:   5 << 20, // top-down PNGs are small
				MimeTypes: []string{"image/png"},
				Protected: false, // game-derived map imagery — served by URL
			},
			&core.TextField{Name: "thumb_status", Max: 16}, // pending/ready/failed/skipped
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)
		// Read for any authed user; mutations flow through the ingest pipeline
		// (app.Save bypasses rules), so create/update/delete stay nil.
		authed := "@request.auth.id != \"\""
		c.ListRule = &authed
		c.ViewRule = &authed
		c.AddIndex("idx_iso_maps_iso", false, "iso", "")
		return app.Save(c)
	}, func(app core.App) error {
		c, err := app.FindCollectionByNameOrId("iso_maps")
		if err != nil {
			return nil
		}
		return app.Delete(c)
	})
}
