package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

// ISO ingest model (design locked with Stewart). The managed library file is
// now named <record-id>.iso — canonical, ID-anchored, decoupled from the freely
// editable display `name`. This adds the fields the ingest + drift-detection
// flow needs:
//
//   - content_hash   sha256 of the managed disc, set at ingest. Dedupe key +
//     the anchor drift-detection re-hashes against.
//   - file_size      managed file size (bytes) — cheap drift pre-check.
//   - file_mtime     managed file mtime (unix seconds) — cheap drift pre-check.
//   - drift_detected the managed bytes no longer match content_hash (tamper /
//     truncation); the row is forced unavailable and flagged.
//
// The old write-once-on-filename constraint (isos_immutable hook) is retired in
// the same change: the ID is the anchor now, so `filename` (repurposed to the
// original inbox filename, for provenance) and `name` are freely editable.
// Additive + reversible.
func init() {
	m.Register(func(app core.App) error {
		isos, err := app.FindCollectionByNameOrId("isos")
		if err != nil {
			return err
		}
		if isos.Fields.GetByName("content_hash") == nil {
			isos.Fields.Add(&core.TextField{Name: "content_hash", Max: 128})
		}
		if isos.Fields.GetByName("file_size") == nil {
			isos.Fields.Add(&core.NumberField{Name: "file_size", OnlyInt: true})
		}
		if isos.Fields.GetByName("file_mtime") == nil {
			isos.Fields.Add(&core.NumberField{Name: "file_mtime", OnlyInt: true})
		}
		if isos.Fields.GetByName("drift_detected") == nil {
			isos.Fields.Add(&core.BoolField{Name: "drift_detected"})
		}
		return app.Save(isos)
	}, func(app core.App) error {
		isos, err := app.FindCollectionByNameOrId("isos")
		if err != nil {
			return err
		}
		for _, f := range []string{"content_hash", "file_size", "file_mtime", "drift_detected"} {
			isos.Fields.RemoveByName(f)
		}
		return app.Save(isos)
	})
}
