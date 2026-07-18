package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// registerAppsCollection creates the `apps` collection — the organizer-curated
// library of homebrew-app ZIP uploads the LAN-sync client installs into the
// Xbox "Apps" folder. It MIRRORS the ISO model (see isos.go): the uploaded
// archive is treated as WRITE-ONCE / immutable, while its metadata stays
// editable, and a derived, rebuildable EXTRACTED cache is produced eagerly on
// upload so headless stations pull an unpacked tree rather than a zip.
//
// Unlike `isos` (a filename CATALOG over a host dir, because a multi-GiB XISO
// can't sensibly round-trip through PB), an app zip is small enough to live in
// PocketBase directly, so `file` is a real FileField byte store.
//
// Immutability is enforced by the apps_immutable hook (block replacing `file`
// after create — replace = delete + new row/ID), NOT by a rule. The extracted
// cache is produced by the apps_extract hook (extract-xiso's sibling: a plain
// unzip) and is evictable/regenerable — it is keyed to this immutable row, so
// no content hash is needed.
//
// Registered from identity.go (phase 6, LAN-sync chain) because the mutate rule
// embeds the organizer/admin user_roles subqueries (phase 1 must run first) and
// sync_presets relates to this collection (must exist before phase-6
// sync_presets).
func registerAppsCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "apps") {
		return reconcileAppsRules(app)
	}

	usersCol, err := requireCollection(app, "users")
	if err != nil {
		return err
	}

	collection := core.NewBaseCollection("apps")
	collection.Fields.Add(
		&core.TextField{Name: "name", Required: true, Min: 1, Max: 120, Presentable: true},
		&core.TextField{Name: "description", Max: 2000},
		// Optional Xbox title ID (hex) — informational metadata for the picker.
		&core.TextField{Name: "title_id", Max: 32},
		// The uploaded app archive (ZIP). IMMUTABLE after create — enforced by
		// the apps_immutable hook, not a rule. Generous cap; homebrew apps are
		// small vs an ISO.
		&core.FileField{
			Name:      "file",
			Required:  true,
			MaxSelect: 1,
			MaxSize:   512 << 20, // 512 MiB
		},
		// Destination folder name under the client's apps dir (SPEC dest_dir =
		// <apps_dir>\<dest_name>, e.g. "\Apps\XBMC4Gamers"). Empty → from name.
		&core.TextField{Name: "dest_name", Max: 64},
		// --- Derived, rebuildable EXTRACTED cache (see apps_extract hook) ---
		// Absolute/relative host path to the unpacked tree for this row. Empty
		// until the extraction hook runs; safe to delete + regenerate. (Apps
		// download as the stored zip; this is used for validation/footprint.)
		&core.TextField{Name: "extracted_path", Max: 1024},
		// Whether the zip validated + footprint computed (served to clients).
		&core.BoolField{Name: "extracted_ready"},
		// When the cache was last (re)built. Nil = never / evicted.
		&core.DateField{Name: "extracted_at"},
		// FATX cluster-rounded uncompressed footprint of the zip (drive-fill
		// math). Computed by the extraction hook. 0 until measured.
		&core.NumberField{Name: "footprint_bytes", OnlyInt: true, Min: f64(0)},
		// Whether players/clients may pull this app (retire without deleting).
		&core.BoolField{Name: "available"},
		&core.RelationField{
			Name:          "created_by",
			CollectionId:  usersCol.Id,
			MaxSelect:     1,
			CascadeDelete: false, // keep the library entry if the author leaves
		},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
	)

	collection.AddIndex("idx_apps_name", false, "name", "")

	setAppsRules(collection)
	return app.Save(collection)
}

func reconcileAppsRules(app *pocketbase.PocketBase) error {
	existing, err := app.FindCollectionByNameOrId("apps")
	if err != nil {
		return err
	}
	setAppsRules(existing)
	return app.Save(existing)
}

// setAppsRules: any authed user may browse + download the library; only
// organizers (or admins) may upload/edit/remove entries. Mirrors game_titles.
func setAppsRules(c *core.Collection) {
	read := "@request.auth.id != \"\""
	mutate := organizerOrAdmin
	c.ListRule = &read
	c.ViewRule = &read
	c.CreateRule = &mutate
	c.UpdateRule = &mutate
	c.DeleteRule = &mutate
}
