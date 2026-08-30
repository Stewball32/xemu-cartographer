package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Nameplates (organizer redesign): the organizer-owned library of 600×100
// (6:1) banner art for the overlay NamePlate. `selectable` gates the player
// picker (a hidden banner stays on players already wearing it — the picker
// filter is the consumer's job); `art` is optional so a named slot can exist
// before its image lands. Players will pick from this library in their own
// settings in a later phase — hence authed read, organizer-or-admin writes.
func init() {
	m.Register(func(app core.App) error {
		if _, err := app.FindCollectionByNameOrId("nameplates"); err == nil {
			return nil // idempotent
		}
		organizer := "((@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"organizer\") || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"))"
		authed := "@request.auth.id != \"\""

		c := core.NewBaseCollection("nameplates")
		c.Fields.Add(
			&core.TextField{Name: "name", Required: true, Min: 1, Max: 48, Presentable: true},
			&core.FileField{
				Name:      "art",
				MaxSelect: 1,
				MaxSize:   5 << 20,
				MimeTypes: []string{"image/png", "image/jpeg", "image/webp"},
			},
			&core.BoolField{Name: "selectable"},
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)
		c.ListRule = &authed
		c.ViewRule = &authed
		c.CreateRule = &organizer
		c.UpdateRule = &organizer
		c.DeleteRule = &organizer
		return app.Save(c)
	}, func(app core.App) error {
		c, err := app.FindCollectionByNameOrId("nameplates")
		if err != nil {
			return nil
		}
		return app.Delete(c)
	})
}
