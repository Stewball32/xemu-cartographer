package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// registerH2ProfilesCollection creates the `h2_profiles` collection — one Halo
// 2 player profile per user, named with the user's gamertag.
//
// Unlike CE, Halo 2 DOES store an editable on-disk player profile (a 500-byte
// `profile` save + its SaveMeta sidecar). So the generate-on-save hook produces
// a real, ready-to-write save bundle: BuildRequest{Title:"h2", Kind:"profile",
// Name: gamertag, Appearance: <appearance bytes>} → internal/saveartifact →
// tar stored in save_bundle. The 20-byte content digest is template-patched
// (preserved) per the LAN-hub generator's documented limitation.
//
// `appearance` is the H2 appearance/controller byte map keyed by
// halosave.H2ProfileFields[].Key (armor colours, emblem, controller bytes).
// Labels are provisional (2-sample reverse engineering) — surfaced as such in
// the editor.
//
// Registered from identity.go (phase 5); relates only to the built-in users
// collection.
func registerH2ProfilesCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "h2_profiles") {
		return reconcileH2ProfilesRules(app)
	}

	usersCol, err := requireCollection(app, "users")
	if err != nil {
		return err
	}

	collection := core.NewBaseCollection("h2_profiles")
	collection.Fields.Add(
		&core.RelationField{
			Name:          "user",
			Required:      true,
			CollectionId:  usersCol.Id,
			MaxSelect:     1,
			CascadeDelete: true,
		},
		&core.TextField{
			Name:        "gamertag",
			Required:    true,
			Min:         1,
			Max:         32,
			Presentable: true,
		},
		// Appearance/controller bytes keyed by halosave field key, each 0..255.
		// Empty {} = keep the template's values (a byte-identical clone of a
		// real profile, just renamed).
		&core.JSONField{Name: "appearance", MaxSize: 1 << 16},
		&core.FileField{
			Name:      "save_bundle",
			MaxSelect: 1,
			MaxSize:   1 << 20,
		},
		&core.JSONField{Name: "save_info", MaxSize: 1 << 16},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
	)

	collection.AddIndex("idx_h2_profiles_user_unique", true, "user", "")

	setH2ProfilesRules(collection)
	return app.Save(collection)
}

func reconcileH2ProfilesRules(app *pocketbase.PocketBase) error {
	existing, err := app.FindCollectionByNameOrId("h2_profiles")
	if err != nil {
		return err
	}
	setH2ProfilesRules(existing)
	return app.Save(existing)
}

// setH2ProfilesRules mirrors ce_profiles: owner reads + mutates their own row,
// admins anything, generated fields written server-side.
func setH2ProfilesRules(c *core.Collection) {
	own := "(@request.auth.id = user) || (" + hasAdminRole + ")"
	c.ListRule = &own
	c.ViewRule = &own
	c.CreateRule = &own
	c.UpdateRule = &own
	c.DeleteRule = &own
}
