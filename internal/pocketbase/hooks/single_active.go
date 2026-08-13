package hooks

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerSingleActiveHooks)
}

// registerSingleActiveHooks enforces "exactly one active" (SPEC §4.2) for
// sync_presets and lan_events: whenever a row is saved with active=true, every
// OTHER row in that collection is demoted to active=false. Runs after the row
// persists (AfterCreateSuccess / AfterUpdateSuccess) so the just-saved active
// row is the survivor. Demoting the others sets active=false, so the demotion
// saves don't re-trigger the cascade — no infinite loop.
//
// This does NOT force one to always exist; it only prevents two being active at
// once. `?preset=active` (and the manifest event lookup) resolve the single
// active row.
func registerSingleActiveHooks(app *pocketbase.PocketBase) {
	for _, collection := range []string{"sync_presets", "lan_events"} {
		col := collection // capture
		demote := func(e *core.RecordEvent) error {
			if err := e.Next(); err != nil {
				return err
			}
			if e.Record.GetBool("active") {
				return demoteOtherActive(e.App, col, e.Record.Id)
			}
			return nil
		}
		app.OnRecordAfterCreateSuccess(col).BindFunc(demote)
		app.OnRecordAfterUpdateSuccess(col).BindFunc(demote)
	}
}

// demoteOtherActive clears active on every row of collection except keepID.
func demoteOtherActive(app core.App, collection, keepID string) error {
	others, err := app.FindRecordsByFilter(collection,
		"active = true && id != {:id}", "", 0, 0, dbx.Params{"id": keepID})
	if err != nil {
		return err
	}
	for _, r := range others {
		r.Set("active", false)
		if err := app.Save(r); err != nil {
			return err
		}
	}
	return nil
}
