package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Adds the optional self-referential `server_iso` relation to the `isos`
// catalog. A playable game IS an isos row (its own file is the game_iso — the
// client build shipped to real Xboxes). It may optionally point `server_iso` at
// another catalog entry that is the dedicated SERVER/host build:
//
//   - host/container provisioning (routes/play → podman) boots server_iso when
//     set, else the game's own file;
//   - the companion-app LAN-sync path (routes/lansync) always serves the game's
//     own build (game_iso), never server_iso.
//
// Additive + reversible. cascadeDelete is false so deleting a server build never
// deletes the games that reference it (the provision path falls back to the
// game's own file for a dangling ref). The neutral-host / host-doesn't-spawn
// flag is deliberately deferred — no tagging here.
func init() {
	m.Register(func(app core.App) error {
		isos, err := app.FindCollectionByNameOrId("isos")
		if err != nil {
			return err
		}
		if isos.Fields.GetByName("server_iso") != nil {
			return nil // idempotent — already applied
		}
		isos.Fields.Add(&core.RelationField{
			Name:          "server_iso",
			CollectionId:  isos.Id, // self-reference into the ISO catalog
			CascadeDelete: false,
			MinSelect:     0,
			MaxSelect:     1,
			Required:      false,
		})
		return app.Save(isos)
	}, func(app core.App) error {
		isos, err := app.FindCollectionByNameOrId("isos")
		if err != nil {
			return err
		}
		isos.Fields.RemoveByName("server_iso")
		return app.Save(isos)
	})
}
