package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// Rulesets (organizer redesign): how a night runs — an ordered set of
// gametypes + a map pool (empty = open pool, organizer picks at the sticks) at
// a team size and series length. Game (ce|h2) is baked at creation so both
// pickers only offer same-game content; Series and stations will reference
// rulesets in a later phase. Unsigned member gametypes bubble a warning in the
// UI — no hard block here, the save may be generated later.
//
// Rules mirror `gametypes`: any authed user reads (the series/stations
// consumers), organizer-or-admin mutates via the PB SDK.
func init() {
	m.Register(func(app core.App) error {
		if _, err := app.FindCollectionByNameOrId("rulesets"); err == nil {
			return nil // idempotent
		}
		gametypes, err := app.FindCollectionByNameOrId("gametypes")
		if err != nil {
			return err
		}
		maps, err := app.FindCollectionByNameOrId("maps")
		if err != nil {
			return err
		}
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			return err
		}

		organizer := "((@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"organizer\") || (@collection.user_roles.user ?= @request.auth.id && @collection.user_roles.role.slug ?= \"admin\"))"
		authed := "@request.auth.id != \"\""

		c := core.NewBaseCollection("rulesets")
		c.Fields.Add(
			&core.TextField{Name: "name", Required: true, Min: 1, Max: 120, Presentable: true},
			&core.SelectField{Name: "game", Values: []string{"ce", "h2"}, MaxSelect: 1, Required: true},
			&core.SelectField{Name: "team_size", Values: []string{"1v1", "2v2", "4v4", "open"}, MaxSelect: 1, Required: true},
			&core.SelectField{Name: "series", Values: []string{"bo1", "bo2", "bo3", "bo4", "bo5", "bo6", "bo7"}, MaxSelect: 1, Required: true},
			&core.RelationField{Name: "gametypes", CollectionId: gametypes.Id, MaxSelect: 99},
			&core.RelationField{Name: "map_pool", CollectionId: maps.Id, MaxSelect: 99},
			&core.TextField{Name: "notes", Max: 2000},
			&core.RelationField{Name: "created_by", CollectionId: users.Id, MaxSelect: 1},
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)
		c.ListRule = &authed
		c.ViewRule = &authed
		c.CreateRule = &organizer
		c.UpdateRule = &organizer
		c.DeleteRule = &organizer
		c.AddIndex("idx_rulesets_game", false, "game", "")
		return app.Save(c)
	}, func(app core.App) error {
		c, err := app.FindCollectionByNameOrId("rulesets")
		if err != nil {
			return nil
		}
		return app.Delete(c)
	})
}
