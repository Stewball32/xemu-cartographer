package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// registerCeProfilesCollection creates the `ce_profiles` collection — one Halo:
// Combat Evolved player profile per user, named with the user's gamertag.
//
// SCAFFOLD (see docs/gamertag-system/README.md). Halo: CE has **no standalone
// multiplayer player-profile save file** (confirmed in the LAN-hub format
// reverse-engineering — the editable CE surface is the gametype, not a profile).
// So this collection exists to hold the gamertag name + a pluggable `settings`
// blob now, and the generate-on-save hook deliberately produces NO save file
// yet: it stamps `save_info.deferred = true`. When the separate CE
// player-name/profile research lands, fill `settings` with the real field set
// and implement generation in internal/saveartifact + the ce_profiles hook —
// every other layer (record, editor, download manifest) is already wired.
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
		// resolved via the `user` relation — not stored here.
		// Pluggable CE customization field set — empty until the CE
		// player-profile research lands. JSON keeps the schema stable while the
		// field set is still being determined.
		&core.JSONField{Name: "settings", MaxSize: 1 << 16},
		// Generated, ready-to-write tar of the FATX save dir. Empty while CE
		// generation is deferred.
		&core.FileField{
			Name:      "save_bundle",
			MaxSelect: 1,
			MaxSize:   1 << 20,
		},
		// Generator metadata (sha1 / sizes / fatx dir / digest status / deferred
		// flag) for display + the download manifest.
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
