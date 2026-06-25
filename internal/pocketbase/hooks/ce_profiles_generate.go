package hooks

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/saveartifact"
)

func init() {
	register(registerCeProfileGenerateHook)
}

// registerCeProfileGenerateHook is the CE side of generate-on-save. On the
// original Xbox, Halo: CE has NO multiplayer player profile / appearance /
// controls — the MP name IS the Xbox console name (E:\UDATA\NICKNAME.XBN). So
// the "CE profile" generates that console-name file from the user's gamertag,
// via the shared internal/consolename builder (the same one the podman
// provisioner writes into a container overlay). One gamertag → both the H2
// profile and this NICKNAME.XBN.
func registerCeProfileGenerateHook(app *pocketbase.PocketBase) {
	app.OnRecordCreate("ce_profiles").BindFunc(generateCeProfile)
	app.OnRecordUpdate("ce_profiles").BindFunc(generateCeProfile)
}

// generateCeProfile builds the NICKNAME.XBN bundle from the user's gamertag and
// attaches it to the record. Exposed as a named function for the integration
// test. A gamertag-less user can't generate one (the save is rejected).
func generateCeProfile(e *core.RecordEvent) error {
	gamertag, err := userGamertag(e.App, e.Record.GetString("user"))
	if err != nil {
		return err
	}
	b, err := saveartifact.CEProfileBundle(gamertag)
	if err != nil {
		return err
	}
	if err := attachResult(e.Record, b, ceProfileFilename(gamertag)); err != nil {
		return err
	}
	return e.Next()
}

func ceProfileFilename(gamertag string) string {
	return "ce-console-name-" + slugFilename(gamertag) + ".tar"
}
