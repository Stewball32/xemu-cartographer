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
const ceProfileDeferredNote = "Halo: CE has player profiles, but the exact profile location + format " +
	"is being re-investigated. This record holds the gamertag + settings now; the generated save file " +
	"is deferred until the format lands. When it does, fill the field set and implement CE generation in " +
	"internal/saveartifact — every other layer is already wired. (The Xbox console name NICKNAME.XBN is a " +
	"separate artifact, not the CE profile.)"

// registerCeProfileGenerateHook handles the CE side of generate-on-save. CE
// profiles are a full profile parallel to H2, but generation is a SCAFFOLD: the
// CE profile-format re-investigation is in progress, so instead of building a
// save it stamps a deferred marker into save_info (and leaves save_bundle empty).
// This keeps the record + editor + manifest fully wired so filling in CE
// generation later is a one-function change (swap the deferred stamp for an
// attachBundle call, like the H2 hook).
func registerCeProfileGenerateHook(app *pocketbase.PocketBase) {
	app.OnRecordCreate("ce_profiles").BindFunc(generateCeProfile)
	app.OnRecordUpdate("ce_profiles").BindFunc(generateCeProfile)
}

// generateCeProfile is the deferred-generation handler, exposed as a named
// function for the integration test. Reads the gamertag from the user relation
// best-effort (for display in save_info — CE has no file to generate yet, so an
// unset gamertag is not fatal here).
func generateCeProfile(e *core.RecordEvent) error {
	gamertag := ""
	if u, err := e.App.FindRecordById("users", e.Record.GetString("user")); err == nil {
		gamertag = u.GetString("gamertag")
	}
	e.Record.Set("save_info", map[string]any{
		"deferred": true,
		"gamertag": gamertag,
		"note":     ceProfileDeferredNote,
	})
	return e.Next()
}
