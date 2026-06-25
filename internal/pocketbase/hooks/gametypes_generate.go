package hooks

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/saveartifact"
)

func init() {
	register(registerGametypeGenerateHook)
}

// registerGametypeGenerateHook regenerates a gametype variant's signed save
// bundle whenever an organizer creates or edits the record. Reads the row's
// title / engine / name / settings and template-patches the matching real
// sample (CE blam.lst or H2 mode-named payload) via internal/saveartifact, then
// stores the deployable tar on save_bundle + metadata on save_info.
//
// Fires before the DB write so the generated fields persist in the same
// transaction. Invalid settings fail the save.
func registerGametypeGenerateHook(app *pocketbase.PocketBase) {
	app.OnRecordCreate("gametypes").BindFunc(generateGametype)
	app.OnRecordUpdate("gametypes").BindFunc(generateGametype)
}

// generateGametype is the create/update handler, exposed as a named function so
// the integration test can bind it directly.
func generateGametype(e *core.RecordEvent) error {
	title := e.Record.GetString("title")
	engine := e.Record.GetString("engine")
	name := e.Record.GetString("name")

	var settings saveartifact.GametypeSettings
	if err := e.Record.UnmarshalJSONField("settings", &settings); err != nil {
		settings = saveartifact.GametypeSettings{}
	}

	req := saveartifact.GametypeRequest(title, engine, name, settings)
	if err := attachBundle(e.Record, req, gametypeFilename(title, name)); err != nil {
		return err
	}
	return e.Next()
}

func gametypeFilename(title, name string) string {
	return title + "-gametype-" + slugFilename(name) + ".tar"
}
