package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerRatingsCollection)
}

// registerRatingsCollection creates the M18b `ratings` store: one row per
// (gamertag, gametype) holding the player's current Elo rating + game count.
// Updated on game-end by the persistence chain (internal/games applies
// internal/rating.Update). No relations and an authed-read / nil-mutate rule,
// so it needs no identity.go coordination — its own init() registers it after
// the FK chains. The M18c leaderboard surfaces read this collection.
func registerRatingsCollection(app *pocketbase.PocketBase) error {
	if existing, err := app.FindCollectionByNameOrId("ratings"); err == nil {
		setRatingsRules(existing)
		return app.Save(existing)
	}

	collection := core.NewBaseCollection("ratings")
	collection.Fields.Add(
		&core.TextField{Name: "gamertag", Required: true, Max: 64, Presentable: true},
		&core.TextField{Name: "gametype", Required: true, Max: 64, Presentable: true},
		&core.NumberField{Name: "rating"},
		&core.NumberField{Name: "games", OnlyInt: true, Min: f64(0)},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
	)

	// One rating per (gamertag, gametype); the leaderboard reads by gametype.
	collection.AddIndex("idx_ratings_gamertag_gametype_unique", true, "gamertag, gametype", "")
	collection.AddIndex("idx_ratings_gametype_rating", false, "gametype, rating", "")

	setRatingsRules(collection)
	return app.Save(collection)
}

// setRatingsRules: authed read (leaderboards / profile pages), nil mutate
// (written by the game-end chain via app.Save, which bypasses rules).
func setRatingsRules(c *core.Collection) {
	authed := "@request.auth.id != \"\""
	c.ListRule = &authed
	c.ViewRule = &authed
	c.CreateRule = nil
	c.UpdateRule = nil
	c.DeleteRule = nil
}
