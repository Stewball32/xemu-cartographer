package hooks

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/saveartifact"
)

func init() {
	register(registerH2ProfileGenerateHook)
}

// registerH2ProfileGenerateHook regenerates a Halo 2 player profile's signed
// save bundle whenever the record is created or edited. The user owns the
// gamertag + appearance bytes; the hub owns the generated file. Fires before
// the DB write (OnRecordCreate / OnRecordUpdate) so the generated save_bundle +
// save_info persist in the same transaction — see attachBundle.
//
// Generation is deterministic template-patch, so regenerating on every save is
// safe and idempotent. An invalid appearance byte fails the save (the generator
// returns an error), which is the intended behaviour: never store an
// unbuildable profile.
func registerH2ProfileGenerateHook(app *pocketbase.PocketBase) {
	app.OnRecordCreate("h2_profiles").BindFunc(generateH2Profile)
	app.OnRecordUpdate("h2_profiles").BindFunc(generateH2Profile)
}

// generateH2Profile is the create/update handler, exposed as a named function
// so the integration test can bind it to a test app directly.
func generateH2Profile(e *core.RecordEvent) error {
	gamertag, err := userGamertag(e.App, e.Record.GetString("user"))
	if err != nil {
		return err
	}

	var appearance map[string]int
	if err := e.Record.UnmarshalJSONField("appearance", &appearance); err != nil {
		// A malformed appearance blob shouldn't hard-fail the save — treat it
		// as "no overrides" (a clean, renamed clone of the template).
		appearance = nil
	}

	req := saveartifact.H2ProfileRequest(gamertag, appearance)
	if err := attachBundle(e.Record, req, h2ProfileFilename(gamertag)); err != nil {
		return err
	}
	return e.Next()
}

func h2ProfileFilename(gamertag string) string {
	return "h2-profile-" + slugFilename(gamertag) + ".tar"
}
