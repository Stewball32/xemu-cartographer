package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// registerGamertagsCollection creates the gamertags collection — one row per
// (user, tag) combo. A user can own multiple tags ("Stewball32" / "Stewball"
// / "Stewie"); the default pick lives on users.default_gamertag.
//
// `blocked` is the soft-moderation flag: admins flip it true when a tag is
// inappropriate. The unique (user, tag) index then prevents the same user
// from resurrecting the same string, while the row stays around as an audit
// trail. Owner update/delete is gated on `blocked = false` via the API rules
// (admins can always edit).
//
// Registration order is controlled by identity.go (not init()) because
// rosters depends on teams and users depends on gamertags.
func registerGamertagsCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "gamertags") {
		return reconcileGamertagsRules(app)
	}

	collection := core.NewBaseCollection("gamertags")

	usersCol, err := requireCollection(app, "users")
	if err != nil {
		return err
	}

	collection.Fields.Add(
		&core.RelationField{
			Name:          "user",
			Required:      true,
			CollectionId:  usersCol.Id,
			MaxSelect:     1,
			CascadeDelete: false,
		},
		&core.TextField{
			Name:        "tag",
			Required:    true,
			Min:         1,
			Max:         32,
			Presentable: true,
		},
		&core.BoolField{
			Name: "blocked",
		},
		&core.AutodateField{
			Name:     "created",
			OnCreate: true,
		},
		&core.AutodateField{
			Name:     "updated",
			OnCreate: true,
			OnUpdate: true,
		},
	)

	collection.AddIndex("idx_gamertags_user_tag_unique", true, "user, tag", "")

	setGamertagsRules(collection)

	return app.Save(collection)
}

func reconcileGamertagsRules(app *pocketbase.PocketBase) error {
	existing, err := app.FindCollectionByNameOrId("gamertags")
	if err != nil {
		return err
	}
	setGamertagsRules(existing)
	return app.Save(existing)
}

// setGamertagsRules:
//   - List/View: any authed user can browse tags (display-only).
//   - Create:    owner adding their own, OR admin acting on behalf.
//   - Update:    owner editing their non-blocked tag, OR admin.
//   - Delete:    admin only — preserves the audit row even after a user
//     "removes" a tag from their settings UI (the UI calls DELETE on rows
//     they hadn't been blocked; backend leaves blocked rows in place).
func setGamertagsRules(c *core.Collection) {
	listView := "@request.auth.id != \"\""
	create := "@request.auth.id = user.id || @request.auth.isAdmin = true"
	update := "(@request.auth.id = user.id && blocked = false) || @request.auth.isAdmin = true"
	del := "(@request.auth.id = user.id && blocked = false) || @request.auth.isAdmin = true"

	c.ListRule = &listView
	c.ViewRule = &listView
	c.CreateRule = &create
	c.UpdateRule = &update
	c.DeleteRule = &del
}
