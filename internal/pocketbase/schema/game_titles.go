package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// registerGameTitlesCollection creates the `game_titles` collection — the
// organizer-curated library of playable game (XBE) uploads the LAN client can
// download to the box.
//
// Named `game_titles`, NOT `games`: the M13 `games` collection already exists
// and records contests played. This is a plain file library (name +
// description + uploaded file), NOT a configurator — there is no generate hook;
// the organizer uploads the bytes directly.
//
// Registered from identity.go (phase 5) because the mutate rule embeds the
// organizer/admin user_roles subqueries.
func registerGameTitlesCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "game_titles") {
		return reconcileGameTitlesRules(app)
	}

	usersCol, err := requireCollection(app, "users")
	if err != nil {
		return err
	}

	collection := core.NewBaseCollection("game_titles")
	collection.Fields.Add(
		&core.TextField{Name: "name", Required: true, Min: 1, Max: 120, Presentable: true},
		&core.TextField{Name: "description", Max: 2000},
		// The uploaded game/XBE file. Generous cap; operators may need to raise
		// PocketBase's request body limit for very large uploads.
		&core.FileField{
			Name:      "file",
			Required:  true,
			MaxSelect: 1,
			MaxSize:   2 << 30, // 2 GiB
		},
		&core.RelationField{
			Name:          "created_by",
			CollectionId:  usersCol.Id,
			MaxSelect:     1,
			CascadeDelete: false,
		},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
	)

	setGameTitlesRules(collection)
	return app.Save(collection)
}

func reconcileGameTitlesRules(app *pocketbase.PocketBase) error {
	existing, err := app.FindCollectionByNameOrId("game_titles")
	if err != nil {
		return err
	}
	setGameTitlesRules(existing)
	return app.Save(existing)
}

// setGameTitlesRules: any authed user may browse + download the library; only
// organizers (or admins) may upload/edit/remove entries.
func setGameTitlesRules(c *core.Collection) {
	read := "@request.auth.id != \"\""
	mutate := organizerOrAdmin
	c.ListRule = &read
	c.ViewRule = &read
	c.CreateRule = &mutate
	c.UpdateRule = &mutate
	c.DeleteRule = &mutate
}
