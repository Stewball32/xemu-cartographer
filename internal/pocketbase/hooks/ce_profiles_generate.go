package hooks

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/saveartifact"
)

func init() {
	register(registerCeProfileGenerateHook)
}

// registerCeProfileGenerateHook is the CE side of generate-on-save. Halo: CE has
// a real player profile (`blam.sav`, parallel to H2) — name + armor color +
// control presets, SIGNED with the per-title HMAC. The hook reads the user's
// gamertag (the in-game name) + the record's `settings` (color / thumbstick /
// button) and generates the signed save bundle via internal/saveartifact →
// internal/halosave.
//
// (The Xbox console name NICKNAME.XBN is a SEPARATE artifact — see the
// /api/lan/saves/console-name endpoint — not this CE profile.)
func registerCeProfileGenerateHook(app *pocketbase.PocketBase) {
	app.OnRecordCreate("ce_profiles").BindFunc(generateCeProfile)
	app.OnRecordUpdate("ce_profiles").BindFunc(generateCeProfile)
}

// generateCeProfile builds the signed blam.sav from the user's gamertag + the
// record's settings and attaches it. Exposed as a named function for the
// integration test. A gamertag-less user can't generate one (the save is
// rejected).
func generateCeProfile(e *core.RecordEvent) error {
	gamertag, err := userGamertag(e.App, e.Record.GetString("user"))
	if err != nil {
		return err
	}

	var settings saveartifact.CEProfileSettings
	if err := e.Record.UnmarshalJSONField("settings", &settings); err != nil {
		settings = saveartifact.CEProfileSettings{}
	}

	req := saveartifact.CEProfileRequest(gamertag, settings)
	if err := attachBundle(e.Record, req, ceProfileFilename(gamertag)); err != nil {
		return err
	}
	return e.Next()
}

func ceProfileFilename(gamertag string) string {
	return "ce-profile-" + slugFilename(gamertag) + ".tar"
}
