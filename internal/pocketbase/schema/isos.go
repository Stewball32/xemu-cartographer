package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerISOsCollection)
}

// registerISOsCollection creates the `isos` collection — the admin-managed
// library of game ISOs available to players when they request an instance.
//
// This is a CATALOG, not a byte store: each row references a bare filename in
// the shared ISO library dir on the host (podman Config.ISODir, default
// containers/xemu/shared/isos). The operator drops the (multi-GiB) XISO into
// that dir; the catalog entry names it + carries player-facing metadata. A
// chosen entry's `filename` feeds podman CreateOptions.GameISO verbatim, which
// bind-mounts the disc read-only and boots the instance straight into that game
// (ADR-0004). Storing the bytes in PocketBase would be wrong — podman needs the
// file on the host to bind-mount, and multi-GiB uploads through PB are
// impractical.
//
// Registered via its own init() (no cross-collection relations, no user_roles
// rule subquery), mirroring containers.go.
//
// REST rules are nil (like `containers`): the collection is reachable only
// through Go routes — admin CRUD at /api/admin/isos/* and the player-scoped
// listing at /api/play/isos — never the auto-generated REST API. That keeps
// unavailable entries and the raw library filenames off the public surface.
func registerISOsCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "isos") {
		return nil
	}

	collection := core.NewBaseCollection("isos")

	collection.Fields.Add(
		// Player-facing display name, e.g. "Halo: Combat Evolved".
		&core.TextField{
			Name:        "name",
			Required:    true,
			Min:         1,
			Max:         120,
			Presentable: true,
		},
		// Bare filename in the shared ISO library (Config.ISODir). No path
		// separators — this is validated in the admin route and resolved against
		// ISODir by podman's resolveGameISO. Feeds CreateOptions.GameISO.
		&core.TextField{
			Name:     "filename",
			Required: true,
			Min:      1,
			Max:      255,
		},
		// Optional Xbox title ID (hex), e.g. "4d530064" — informational metadata
		// the picker / debug views can show.
		&core.TextField{
			Name: "title_id",
			Max:  32,
		},
		&core.TextField{
			Name: "description",
			Max:  2000,
		},
		// Whether players may pick this ISO. An admin can keep a broken/retired
		// disc in the catalog but hidden from the player picker. The player list
		// (/api/play/isos) filters on this; admin CRUD sees everything.
		&core.BoolField{
			Name: "available",
		},
		&core.AutodateField{Name: "created", OnCreate: true},
		&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
	)

	// One catalog entry per library file.
	collection.AddIndex("idx_isos_filename_unique", true, "filename", "")

	// Admin-route + play-route (Go) access only — no direct REST.
	collection.ListRule = nil
	collection.ViewRule = nil
	collection.CreateRule = nil
	collection.UpdateRule = nil
	collection.DeleteRule = nil

	return app.Save(collection)
}
