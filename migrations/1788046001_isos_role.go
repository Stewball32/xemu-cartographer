package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Disc roles (organizer redesign): the boolean `available` toggle becomes a
// three-state `role` — play (shows in player-facing pickers), server (boots the
// xemu-cart host instance, never shown to players), shelved (in the library,
// hidden everywhere) — plus `allow_on_xbox`, which marks a disc eligible for
// real-Xbox station HDDs regardless of role (the actual push selection happens
// at sync time via lansync presets, which reference discs explicitly).
//
// Backfill: a disc referenced as some other disc's server_iso → server;
// otherwise available:true → play, available:false → shelved. allow_on_xbox
// inherits the old available value (available discs were both playable and
// station-synced under the one-toggle model). New ingests land shelved
// (internal/isoingest sets the default).
func init() {
	m.Register(func(app core.App) error {
		isos, err := app.FindCollectionByNameOrId("isos")
		if err != nil {
			return err
		}
		if isos.Fields.GetByName("role") == nil {
			isos.Fields.Add(&core.SelectField{
				Name:      "role",
				Values:    []string{"play", "server", "shelved"},
				MaxSelect: 1,
			})
		}
		if isos.Fields.GetByName("allow_on_xbox") == nil {
			isos.Fields.Add(&core.BoolField{Name: "allow_on_xbox"})
		}
		if err := app.Save(isos); err != nil {
			return err
		}

		records, err := app.FindAllRecords("isos")
		if err != nil {
			return err
		}
		serverIDs := map[string]bool{}
		for _, r := range records {
			if id := r.GetString("server_iso"); id != "" {
				serverIDs[id] = true
			}
		}
		for _, r := range records {
			avail := r.GetBool("available")
			role := "shelved"
			switch {
			case serverIDs[r.Id]:
				role = "server"
			case avail:
				role = "play"
			}
			r.Set("role", role)
			r.Set("allow_on_xbox", avail)
			if err := app.Save(r); err != nil {
				return err
			}
		}

		isos.Fields.RemoveByName("available")
		return app.Save(isos)
	}, func(app core.App) error {
		isos, err := app.FindCollectionByNameOrId("isos")
		if err != nil {
			return err
		}
		if isos.Fields.GetByName("available") == nil {
			isos.Fields.Add(&core.BoolField{Name: "available"})
		}
		if err := app.Save(isos); err != nil {
			return err
		}
		records, err := app.FindAllRecords("isos")
		if err != nil {
			return err
		}
		for _, r := range records {
			r.Set("available", r.GetString("role") == "play")
			if err := app.Save(r); err != nil {
				return err
			}
		}
		isos.Fields.RemoveByName("role")
		isos.Fields.RemoveByName("allow_on_xbox")
		return app.Save(isos)
	})
}
