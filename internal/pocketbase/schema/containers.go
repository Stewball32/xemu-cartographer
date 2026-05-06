package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerContainersCollection)
}

func registerContainersCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "containers") {
		return nil
	}

	collection := core.NewBaseCollection("containers")

	collection.Fields.Add(
		&core.TextField{
			Name:        "name",
			Required:    true,
			Min:         1,
			Max:         64,
			Presentable: true,
		},
		&core.NumberField{
			Name:    "index",
			OnlyInt: true,
			Min:     f64(0),
		},
		&core.NumberField{
			Name:     "xemu_http",
			Required: true,
			OnlyInt:  true,
			Min:      f64(0),
		},
		&core.NumberField{
			Name:     "xemu_https",
			Required: true,
			OnlyInt:  true,
			Min:      f64(0),
		},
		&core.NumberField{
			Name:     "xemu_ws",
			Required: true,
			OnlyInt:  true,
			Min:      f64(0),
		},
		&core.NumberField{
			Name:     "browser_web",
			Required: true,
			OnlyInt:  true,
			Min:      f64(0),
		},
		&core.NumberField{
			Name:     "browser_vnc",
			Required: true,
			OnlyInt:  true,
			Min:      f64(0),
		},
		&core.DateField{
			Name: "created",
		},
		&core.TextField{
			Name:   "vnc_password",
			Hidden: true,
			Max:    128,
		},
	)

	collection.AddIndex("idx_containers_name_unique", true, "name", "")
	collection.AddIndex("idx_containers_index_unique", true, "index", "")

	// Admin-only via /api/admin/containers/* — no direct REST collection access.
	collection.ListRule = nil
	collection.ViewRule = nil
	collection.CreateRule = nil
	collection.UpdateRule = nil
	collection.DeleteRule = nil

	return app.Save(collection)
}
