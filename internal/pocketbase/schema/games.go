package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// Registration is coordinated from identity.go (M13): games relates to series,
// so series must register first. games.go exposes only the register function.

// registerGamesCollection creates the M13 `games` collection — one singular
// contest (one round of Slayer, one CTF round) belonging to a series. Written
// on the scraper's Live→Ready transition (the path that already captures
// `cache.PreviousGame`); the per-player breakdown lives in game_players.
//
// snapshot_blob is an optional trimmed instanceCache JSON for replay/debug;
// the canonical per-tick event stream stays in the existing instance-keyed
// game_events firehose (the game↔events linkage is a deferred M13 decision —
// see the milestone Log).
func registerGamesCollection(app *pocketbase.PocketBase) error {
	if existing, err := app.FindCollectionByNameOrId("games"); err == nil {
		setGamesRules(existing)
		return app.Save(existing)
	}

	seriesCol, err := app.FindCollectionByNameOrId("series")
	if err != nil {
		return err
	}

	collection := core.NewBaseCollection("games")
	collection.Fields.Add(
		&core.RelationField{Name: "series", Required: true, CollectionId: seriesCol.Id, MaxSelect: 1},
		&core.TextField{Name: "container", Max: 64},
		&core.TextField{Name: "host_machine_name", Max: 64},
		&core.TextField{Name: "map", Max: 64, Presentable: true},
		&core.TextField{Name: "gametype", Max: 64},
		&core.TextField{Name: "variant_name", Max: 128},
		&core.DateField{Name: "started_at"},
		&core.DateField{Name: "ended_at"},
		&core.NumberField{Name: "winner_team", OnlyInt: true},
		&core.TextField{Name: "score_summary", Max: 256},
		&core.JSONField{Name: "snapshot_blob", MaxSize: 1 << 20},
		&core.AutodateField{Name: "created", OnCreate: true},
	)

	collection.AddIndex("idx_games_series", false, "series", "")
	collection.AddIndex("idx_games_ended_at", false, "ended_at", "")

	setGamesRules(collection)
	return app.Save(collection)
}

func setGamesRules(c *core.Collection) {
	authed := "@request.auth.id != \"\""
	c.ListRule = &authed
	c.ViewRule = &authed
	c.CreateRule = nil
	c.UpdateRule = nil
	c.DeleteRule = nil
}
