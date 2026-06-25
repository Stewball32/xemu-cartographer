package hooks

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerCeProfileGenerateHook)
}

// ceProfileDeferredNote is stamped into a CE profile's save_info so the editor
// can plainly show that no save file is generated yet, and WHY.
const ceProfileDeferredNote = "Halo: CE has no standalone multiplayer player-profile save file " +
	"(the editable CE surface is the gametype). This profile stores the gamertag + settings " +
	"now; the generated save file is deferred pending the CE player-name/profile research. " +
	"When that lands, implement CE profile generation in internal/saveartifact and switch this " +
	"hook to attachBundle — every other layer is already wired."

// registerCeProfileGenerateHook handles the CE side of generate-on-save. CE
// generation is a SCAFFOLD: there is no CE profile file format yet, so instead
// of building a save it stamps a deferred marker into save_info (and leaves
// save_bundle empty). This keeps the record + editor + download manifest fully
// wired so that filling in CE generation later is a one-function change.
//
// The single seam: when CE profile generation is implemented, replace the
// deferred stamp with the same attachBundle(...) call the H2 hook uses.
func registerCeProfileGenerateHook(app *pocketbase.PocketBase) {
	gen := func(e *core.RecordEvent) error {
		e.Record.Set("save_info", map[string]any{
			"deferred": true,
			"gamertag": e.Record.GetString("gamertag"),
			"note":     ceProfileDeferredNote,
		})
		return e.Next()
	}

	app.OnRecordCreate("ce_profiles").BindFunc(gen)
	app.OnRecordUpdate("ce_profiles").BindFunc(gen)
}
