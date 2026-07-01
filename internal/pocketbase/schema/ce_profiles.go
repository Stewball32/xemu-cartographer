package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// registerCeProfilesCollection creates the `ce_profiles` collection — one Halo:
// Combat Evolved player profile per user, a FULL profile parallel to h2_profiles.
//
// CE has a real player profile (`blam.sav`, format cracked — see
// docs/gamertag-system/CE-PROFILE-FORMAT.md): name (from the user's gamertag) +
// armor color + control presets, SIGNED. The generate-on-save hook builds the
// signed bundle (`UDATA/4d530004/<id>/{blam.sav, SaveMeta.xbx}`) via
// internal/saveartifact → internal/halosave. `settings` holds the editable
// fields (`color` / `thumbstick` / `button`; the advanced 0x1C-0x2F bytes are a
// pluggable follow-up).
//
// NOTE: the Xbox console name (E:\UDATA\NICKNAME.XBN) is a SEPARATE artifact
// (dashboard / system-link identity), NOT the CE player profile — see the
// stateless /api/lan/saves/console-name/{gamertag} endpoint.
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
		// resolved via the `user` relation. `settings` holds the editable CE
		// fields: color / thumbstick / button (+ future advanced bytes).
		&core.JSONField{Name: "settings", MaxSize: 1 << 16},
		// Generated, ready-to-write tar of the FATX save dir (blam.sav + SaveMeta).
		&core.FileField{
			Name:      "save_bundle",
			MaxSelect: 1,
			MaxSize:   1 << 20,
		},
		// Generator metadata (sha1 / sizes / fatx dir / digest status).
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
