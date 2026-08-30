package hooks

import (
	"errors"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerMapsVariantGuardHook)
}

// registerMapsVariantGuardHook enforces the Maps catalog's variant invariants
// (the PB rule layer can't express cross-record checks):
//
//   - variant_of must point at a SAME-GAME map (a CE retune can't nest under an
//     H2 map),
//   - the target may not itself be a variant — variants can't be variant
//     targets, so chains can't form (Maps board 1a),
//   - and a map that HAS variants can't become one (the same chain, approached
//     from the other end),
//   - self-reference is out.
//
// Fires on update only — catalog rows are minted bare by the ingest sync.
func registerMapsVariantGuardHook(app *pocketbase.PocketBase) {
	app.OnRecordUpdate("maps").BindFunc(func(e *core.RecordEvent) error {
		targetID := e.Record.GetString("variant_of")
		if targetID == "" {
			return e.Next()
		}
		if targetID == e.Record.Id {
			return errors.New("a map can't be a variant of itself")
		}
		target, err := e.App.FindRecordById("maps", targetID)
		if err != nil {
			return errors.New("variant_of must reference an existing map")
		}
		if target.GetString("game") != e.Record.GetString("game") {
			return errors.New("variant_of must reference a map from the same game")
		}
		if target.GetString("variant_of") != "" {
			return errors.New("variants can't be variant targets — pick the original map")
		}
		children, err := e.App.FindRecordsByFilter("maps", "variant_of = {:id}", "", 1, 0,
			dbx.Params{"id": e.Record.Id})
		if err == nil && len(children) > 0 {
			return errors.New("this map has variants of its own — reassign them before making it a variant")
		}
		return e.Next()
	})
}
