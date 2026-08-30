package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Canonical map catalog (organizer redesign, Maps page). A row is a unique
// BUILD — (game, filename, content_hash) — not a filename: a modded disc that
// ships a different damnation.map under the stock name gets its own row, and
// byte-identical caches across discs collapse into one. Rows are minted by the
// ingest pipeline / startup backfill (never by hand); the organizer then
// curates identity on top: display_name, variant_of (no chains — a variant
// can't be a variant target, enforced by the maps_variant_guard hook),
// description, an uploaded in-game graphic (falls back to the iso_maps BSP
// render in the UI), and the power_items rotation table
// ([{items:["Rockets"],every:"2:00"}] — multi-item rows alternate each spawn).
//
// iso_maps additionally gains `content_hash` so per-disc cache rows can be
// grouped under their catalog build (computed at ingest; backfilled for
// pre-existing rows by hashing the extracted tree on boot).
//
// Rules: read for any authed user (rulesets pickers + organizer). Update is
// organizer-or-admin (PB SDK PATCH incl. the graphic upload); create/delete
// stay nil — rows are server-minted and die only with their builds.
func init() {
	m.Register(func(app core.App) error {
		if iso, err := app.FindCollectionByNameOrId("iso_maps"); err == nil {
			if iso.Fields.GetByName("content_hash") == nil {
				iso.Fields.Add(&core.TextField{Name: "content_hash", Max: 64})
				if err := app.Save(iso); err != nil {
					return err
				}
			}
		}

		if _, err := app.FindCollectionByNameOrId("maps"); err == nil {
			return nil // idempotent
		}
		organizer := "((@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"organizer\") || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"))"
		authed := "@request.auth.id != \"\""

		c := core.NewBaseCollection("maps")
		c.Fields.Add(
			&core.TextField{Name: "filename", Required: true, Max: 128},
			&core.TextField{Name: "content_hash", Required: true, Max: 64},
			&core.SelectField{Name: "game", Values: []string{"ce", "h2"}, MaxSelect: 1, Required: true},
			&core.TextField{Name: "display_name", Max: 128},
			&core.TextField{Name: "description", Max: 2000},
			&core.FileField{
				Name:      "graphic",
				MaxSelect: 1,
				MaxSize:   10 << 20,
				MimeTypes: []string{"image/png", "image/jpeg", "image/webp"},
			},
			&core.JSONField{Name: "power_items", MaxSize: 1 << 20},
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)
		c.ListRule = &authed
		c.ViewRule = &authed
		c.UpdateRule = &organizer
		c.AddIndex("idx_maps_build", true, "game, filename, content_hash", "")
		if err := app.Save(c); err != nil {
			return err
		}

		// Self-relation needs the collection's own id, so it lands in a second
		// save. Nullify on delete (CascadeDelete:false) — deleting a parent
		// leaves ex-variants standalone rather than deleting them.
		saved, err := app.FindCollectionByNameOrId("maps")
		if err != nil {
			return err
		}
		saved.Fields.Add(&core.RelationField{
			Name:         "variant_of",
			CollectionId: saved.Id,
			MaxSelect:    1,
		})
		return app.Save(saved)
	}, func(app core.App) error {
		if c, err := app.FindCollectionByNameOrId("maps"); err == nil {
			if err := app.Delete(c); err != nil {
				return err
			}
		}
		if iso, err := app.FindCollectionByNameOrId("iso_maps"); err == nil {
			iso.Fields.RemoveByName("content_hash")
			return app.Save(iso)
		}
		return nil
	})
}
