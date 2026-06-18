package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// Registration is coordinated from identity.go (M13): the games-persistence FK
// chain is series → games → game_players, and per-file init()s run in
// alphabetical filename order ("game_players" < "games" < "series"), which is
// the reverse of the dependency order. series.go therefore exposes only the
// register function; identity.go calls it first.

// registerSeriesCollection creates the M13 `series` collection — a grouping of
// one or more games with a format + category. A pickup match is a 1-game
// series; a tournament round is a best-of-N / first-to-X series. Category is
// defaulted from the gametype variant-name heuristic (internal/games) and is
// editable after the fact. "In progress" is modelled as the absence of
// `ended_at`.
func registerSeriesCollection(app *pocketbase.PocketBase) error {
	if existing, err := app.FindCollectionByNameOrId("series"); err == nil {
		setSeriesRules(existing)
		return app.Save(existing)
	}

	usersCol, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}

	collection := core.NewBaseCollection("series")
	collection.Fields.Add(
		&core.TextField{Name: "name", Max: 128, Presentable: true},
		&core.SelectField{
			Name:      "format",
			Required:  true,
			MaxSelect: 1,
			// Mirrors internal/series format constants (M14 widens the set).
			Values: []string{"single", "exact-n", "best-of-n", "first-to-x"},
		},
		&core.NumberField{Name: "target_n", OnlyInt: true, Min: f64(1)},
		&core.SelectField{
			Name:      "category",
			Required:  true,
			MaxSelect: 1,
			Values:    []string{"casual", "competitive", "tournament", "custom"},
		},
		&core.RelationField{Name: "created_by", CollectionId: usersCol.Id, MaxSelect: 1},
		&core.DateField{Name: "started_at"},
		&core.DateField{Name: "ended_at"},
		// tournament + tournament_round are M16 placeholders — a series that
		// belongs to a tournament round. Free-form for now; M16 formalizes.
		&core.TextField{Name: "tournament", Max: 64},
		&core.NumberField{Name: "tournament_round", OnlyInt: true, Min: f64(0)},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
	)

	collection.AddIndex("idx_series_category_created", false, "category, created", "")

	setSeriesRules(collection)
	return app.Save(collection)
}

// setSeriesRules: authed read (stats/series UIs in M14/M15 list these), nil
// mutate — series rows are written by internal/games via app.Save (bypasses
// rules) and, later, the M14 series-setup routes.
func setSeriesRules(c *core.Collection) {
	authed := "@request.auth.id != \"\""
	c.ListRule = &authed
	c.ViewRule = &authed
	c.CreateRule = nil
	c.UpdateRule = nil
	c.DeleteRule = nil
}
