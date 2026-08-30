package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Stream identity (settings redesign, Stream tab): the two nameplate fields the
// overlays read through /api/public/profiles — `motto` (the plate's second
// line, big plates only) and `nameplate` (the organizer-curated banner the
// player picked; players choose from selectable rows, they never upload —
// that's the organizer's Nameplates page). Nullify on delete: retiring a
// banner leaves pickers empty rather than cascading into user rows.
func init() {
	m.Register(func(app core.App) error {
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}
		nameplates, err := app.FindCollectionByNameOrId("nameplates")
		if err != nil {
			return err
		}
		if users.Fields.GetByName("motto") == nil {
			users.Fields.Add(&core.TextField{Name: "motto", Max: 40})
		}
		if users.Fields.GetByName("nameplate") == nil {
			users.Fields.Add(&core.RelationField{
				Name:         "nameplate",
				CollectionId: nameplates.Id,
				MaxSelect:    1,
			})
		}
		return app.Save(users)
	}, func(app core.App) error {
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}
		users.Fields.RemoveByName("motto")
		users.Fields.RemoveByName("nameplate")
		return app.Save(users)
	})
}
