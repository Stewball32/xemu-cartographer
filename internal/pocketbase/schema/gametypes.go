package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// registerGametypesCollection creates the `gametypes` collection — the shared,
// organizer-curated library of Halo: CE and Halo 2 multiplayer gametype
// variants every player can download to their box.
//
// Distinct from the M13 `games` collection (which records contests played).
// One row = one gametype variant (e.g. "Team Slayer 50"). The generate-on-save
// hook turns the row's `title` / `engine` / `name` / `settings` into a real,
// ready-to-write save bundle via internal/saveartifact (CE blam.lst or H2
// mode-named payload, each with its SaveMeta sidecar), stored in save_bundle.
//
// Registered from identity.go (phase 5) because the mutate rule embeds the
// organizer/admin user_roles subqueries. Relates only to the built-in users
// collection (created_by).
func registerGametypesCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "gametypes") {
		return reconcileGametypesRules(app)
	}

	usersCol, err := requireCollection(app, "users")
	if err != nil {
		return err
	}

	collection := core.NewBaseCollection("gametypes")
	collection.Fields.Add(
		&core.SelectField{
			Name:      "title",
			Required:  true,
			Values:    []string{"ce", "h2"},
			MaxSelect: 1,
		},
		// CE engine (slayer/ctf/oddball/king/race) or H2 mode (slayer). Selects
		// the template the generator patches.
		&core.TextField{Name: "engine", Required: true, Min: 1, Max: 32},
		&core.TextField{Name: "name", Required: true, Min: 1, Max: 64, Presentable: true},
		// Gametype parameters mirroring the halosave BuildRequest subset for the
		// title (CE: teams/radar/score_limit/time_minutes/...; H2: score_limit).
		&core.JSONField{Name: "settings", MaxSize: 1 << 16},
		&core.FileField{
			Name:      "save_bundle",
			MaxSelect: 1,
			MaxSize:   1 << 20,
		},
		&core.JSONField{Name: "save_info", MaxSize: 1 << 16},
		&core.RelationField{
			Name:          "created_by",
			CollectionId:  usersCol.Id,
			MaxSelect:     1,
			CascadeDelete: false, // keep the library entry if the author leaves
		},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
	)

	collection.AddIndex("idx_gametypes_title", false, "title", "")

	setGametypesRules(collection)
	return app.Save(collection)
}

func reconcileGametypesRules(app *pocketbase.PocketBase) error {
	existing, err := app.FindCollectionByNameOrId("gametypes")
	if err != nil {
		return err
	}
	setGametypesRules(existing)
	return app.Save(existing)
}

// setGametypesRules: any authed user may read + download the shared library;
// only organizers (or admins) may add/edit/remove entries.
func setGametypesRules(c *core.Collection) {
	read := "@request.auth.id != \"\""
	mutate := organizerOrAdmin
	c.ListRule = &read
	c.ViewRule = &read
	c.CreateRule = &mutate
	c.UpdateRule = &mutate
	c.DeleteRule = &mutate
}
