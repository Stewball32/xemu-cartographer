package hooks

import (
	"errors"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func init() {
	register(registerUsersNameplateGuardHook)
}

// registerUsersNameplateGuardHook enforces the nameplate-picking rule from the
// organizer redesign: players pick from the banners the organizer marked
// SELECTABLE. A newly-assigned banner must exist and be selectable; a banner
// the player already wears stays valid if the organizer later hides it
// ("current wearers keep it" — only the picker shrinks). Clearing is always
// allowed. Cross-record checks like this can't live in a PB rule, hence a hook
// (same reasoning as maps_variant_guard).
func registerUsersNameplateGuardHook(app *pocketbase.PocketBase) {
	app.OnRecordUpdate("users").BindFunc(guardUserNameplate)
}

// guardUserNameplate is the handler, exposed as a named function for the
// integration test.
func guardUserNameplate(e *core.RecordEvent) error {
	cur := e.Record.GetString("nameplate")
	if cur == "" {
		return e.Next() // clearing (or untouched empty) is always fine
	}
	old := ""
	if orig := e.Record.Original(); orig != nil {
		old = orig.GetString("nameplate")
	}
	if cur == old {
		return e.Next() // unchanged — hidden-but-worn banners stay put
	}
	plate, err := e.App.FindRecordById("nameplates", cur)
	if err != nil {
		return errors.New("that banner doesn't exist")
	}
	if !plate.GetBool("selectable") {
		return errors.New("that banner isn't selectable")
	}
	return e.Next()
}
