package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// Registration is coordinated from identity.go (M13): game_players relates to
// games, so games must register first. Exposes only the register function.

// registerGamePlayersCollection creates the M13 `game_players` collection —
// one row per player per game, the per-game stat breakdown that M15
// per-user / per-team stats aggregate. `gamertag` is stored as the raw string
// (matched to a gamertags row later) so a stat survives a gamertag rename or
// a player who never registered an account.
func registerGamePlayersCollection(app *pocketbase.PocketBase) error {
	if existing, err := app.FindCollectionByNameOrId("game_players"); err == nil {
		setGamePlayersRules(existing)
		return app.Save(existing)
	}

	gamesCol, err := app.FindCollectionByNameOrId("games")
	if err != nil {
		return err
	}

	collection := core.NewBaseCollection("game_players")
	collection.Fields.Add(
		&core.RelationField{Name: "game", Required: true, CollectionId: gamesCol.Id, MaxSelect: 1},
		&core.TextField{Name: "gamertag", Required: true, Max: 64, Presentable: true},
		&core.NumberField{Name: "team", OnlyInt: true},
		&core.NumberField{Name: "kills", OnlyInt: true},
		&core.NumberField{Name: "deaths", OnlyInt: true},
		&core.NumberField{Name: "assists", OnlyInt: true},
		&core.NumberField{Name: "score", OnlyInt: true},
		&core.NumberField{Name: "time_alive_ms", OnlyInt: true, Min: f64(0)},
		&core.JSONField{Name: "weapon_loadout", MaxSize: 1 << 16},
		&core.AutodateField{Name: "created", OnCreate: true},
	)

	collection.AddIndex("idx_game_players_game", false, "game", "")
	collection.AddIndex("idx_game_players_gamertag", false, "gamertag", "")

	setGamePlayersRules(collection)
	return app.Save(collection)
}

func setGamePlayersRules(c *core.Collection) {
	authed := "@request.auth.id != \"\""
	c.ListRule = &authed
	c.ViewRule = &authed
	c.CreateRule = nil
	c.UpdateRule = nil
	c.DeleteRule = nil
}
