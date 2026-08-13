package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Offset versioning (Stewart's model): memory offsets live in version-level
// config files (internal/scraper/offsets/sets/*.json), one baseline per game.
// This adds the SELECTION field: `offset_set` on `isos` names the offset-set id
// a build's scraper should bind ("" = the detected game's baseline). Assigning
// a build to an existing set is pure data (no redeploy); authoring a NEW set is
// a new config file + deploy. Additive + reversible.
func init() {
	m.Register(func(app core.App) error {
		isos, err := app.FindCollectionByNameOrId("isos")
		if err != nil {
			return err
		}
		if isos.Fields.GetByName("offset_set") == nil {
			isos.Fields.Add(&core.TextField{Name: "offset_set", Max: 64})
		}
		return app.Save(isos)
	}, func(app core.App) error {
		isos, err := app.FindCollectionByNameOrId("isos")
		if err != nil {
			return err
		}
		isos.Fields.RemoveByName("offset_set")
		return app.Save(isos)
	})
}
