package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// registerCeProfilesCollection creates the `ce_profiles` collection — one Halo:
// Combat Evolved "profile" per user.
//
// NAME-ONLY (see docs/gamertag-system/README.md). On the original Xbox, Halo: CE
// has **no multiplayer player profile / appearance / controls** — the MP name
// comes solely from the Xbox console name E:\UDATA\NICKNAME.XBN (the "name +
// armor + controls" profile is Halo PC, not Xbox CE). So this record has no
// editable fields: the generate-on-save hook builds NICKNAME.XBN from the user's
// gamertag (via internal/consolename) and stores it on save_bundle. The record
// exists so each user has a CE profile alongside their H2 one, regeneration
// cascades on gamertag change, and the LAN manifest can serve it.
//
// Registered from identity.go (phase 5) because its rules embed the
// hasAdminRole subquery, which requires user_roles to exist at rule-validation
// time. Relates only to the built-in users collection.
func registerCeProfilesCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "ce_profiles") {
		return reconcileCeProfilesRules(app)
	}

	usersCol, err := requireCollection(app, "users")
	if err != nil {
		return err
	}

	collection := core.NewBaseCollection("ce_profiles")
	collection.Fields.Add(
		&core.RelationField{
			Name:          "user",
			Required:      true,
			CollectionId:  usersCol.Id,
			MaxSelect:     1,
			CascadeDelete: true, // a user's profile dies with the user
		},
		// The in-game name comes from users.gamertag (single source of truth),
		// resolved via the `user` relation. No other editable fields — CE is
		// name-only. The generated, ready-to-write tar (UDATA/NICKNAME.XBN):
		&core.FileField{
			Name:      "save_bundle",
			MaxSelect: 1,
			MaxSize:   1 << 20,
		},
		// Generator metadata (sha1 / sizes / fatx dir) for display + the manifest.
		&core.JSONField{Name: "save_info", MaxSize: 1 << 16},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
	)

	collection.AddIndex("idx_ce_profiles_user_unique", true, "user", "")

	setCeProfilesRules(collection)
	return app.Save(collection)
}

func reconcileCeProfilesRules(app *pocketbase.PocketBase) error {
	existing, err := app.FindCollectionByNameOrId("ce_profiles")
	if err != nil {
		return err
	}
	setCeProfilesRules(existing)
	return app.Save(existing)
}

// setCeProfilesRules: a user reads + mutates their own profile; admins can do
// anything. The save_bundle / save_info fields are written server-side by the
// generate-on-save hook, not the client. The LAN download manifest reads
// profiles through e.App (superuser context), so these rules don't gate the
// device client.
func setCeProfilesRules(c *core.Collection) {
	own := "(@request.auth.id = user) || (" + hasAdminRole + ")"
	c.ListRule = &own
	c.ViewRule = &own
	c.CreateRule = &own
	c.UpdateRule = &own
	c.DeleteRule = &own
}
