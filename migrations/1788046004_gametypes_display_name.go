package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Two-name gametypes (organizer redesign, Gametypes page): `name` stays the
// library label (what rulesets and the organizer list show); the new
// `display_name` is the IN-GAME name — written into the signed save, shown on
// the pregame lobby list (CE truncates past 11 chars there, which the editor
// warns about). Backfill: existing variants used one name for both roles, so
// display_name starts as a copy. The gametypes_generate hook prefers
// display_name (falling back to name) when patching the template save.
func init() {
	m.Register(func(app core.App) error {
		c, err := app.FindCollectionByNameOrId("gametypes")
		if err != nil {
			return err
		}
		if c.Fields.GetByName("display_name") == nil {
			c.Fields.Add(&core.TextField{Name: "display_name", Max: 64})
			if err := app.Save(c); err != nil {
				return err
			}
		}
		records, err := app.FindAllRecords("gametypes")
		if err != nil {
			return err
		}
		for _, r := range records {
			if r.GetString("display_name") == "" {
				r.Set("display_name", r.GetString("name"))
				if err := app.Save(r); err != nil {
					return err
				}
			}
		}
		return nil
	}, func(app core.App) error {
		c, err := app.FindCollectionByNameOrId("gametypes")
		if err != nil {
			return err
		}
		c.Fields.RemoveByName("display_name")
		return app.Save(c)
	})
}
