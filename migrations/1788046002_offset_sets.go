package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Runtime offset sets (organizer redesign, Offsets page): imported offsetmap
// JSON exports from the hunting rig land as records here — `set_id` is the
// stable id discs bind (isos.offset_set), `file` holds the export byte-identical
// for re-download, `version` bumps when the same set_id is re-imported. The
// embedded baselines (internal/scraper/offsets/sets/) stay compiled in and are
// NOT mirrored here; the listing endpoint merges both worlds and the scraper's
// resolver falls through to this collection for non-embedded ids.
//
// Server-managed: nil rules — all access flows through the
// /api/admin/isos/offset-sets routes (organizer-or-admin), which validate the
// upload and handle delete-with-migration.
func init() {
	m.Register(func(app core.App) error {
		if _, err := app.FindCollectionByNameOrId("offset_sets"); err == nil {
			return nil // idempotent
		}
		c := core.NewBaseCollection("offset_sets")
		c.Fields.Add(
			&core.TextField{Name: "set_id", Required: true, Max: 64},
			&core.TextField{Name: "game", Required: true, Max: 32},
			&core.TextField{Name: "description", Max: 500},
			&core.FileField{
				Name:      "file",
				MaxSelect: 1,
				MaxSize:   5 << 20,
				MimeTypes: []string{"application/json", "text/plain"},
			},
			&core.NumberField{Name: "count", OnlyInt: true},
			&core.NumberField{Name: "version", OnlyInt: true},
			// original upload filename — provenance only ("source" in the UI).
			&core.TextField{Name: "source_name", Max: 128},
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)
		c.AddIndex("idx_offset_sets_set_id", true, "set_id", "")
		return app.Save(c)
	}, func(app core.App) error {
		c, err := app.FindCollectionByNameOrId("offset_sets")
		if err != nil {
			return nil
		}
		return app.Delete(c)
	})
}
