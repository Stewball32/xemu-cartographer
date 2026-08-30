package hooks

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// Integration tests for the two stream-identity user hooks: the default-
// gamertag → users.gamertag sync (which is what cascades profile regen) and
// the nameplate selectable guard. Real test PB app; the handlers are bound
// directly (the app-level register path needs a *pocketbase.PocketBase).

func streamIdentityApp(t *testing.T) core.App {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)

	usersCol, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatalf("users collection: %v", err)
	}
	for _, f := range []core.Field{
		&core.TextField{Name: "gamertag", Max: 11},
		&core.TextField{Name: "default_gamertag", Max: 32},
		&core.TextField{Name: "nameplate", Max: 32},
	} {
		if usersCol.Fields.GetByName(f.GetName()) == nil {
			usersCol.Fields.Add(f)
		}
	}
	if err := app.Save(usersCol); err != nil {
		t.Fatalf("extend users: %v", err)
	}

	gts := core.NewBaseCollection("gamertags")
	gts.Fields.Add(
		&core.TextField{Name: "tag", Max: 32},
		&core.TextField{Name: "status", Max: 16},
	)
	if err := app.Save(gts); err != nil {
		t.Fatalf("save gamertags: %v", err)
	}

	plates := core.NewBaseCollection("nameplates")
	plates.Fields.Add(
		&core.TextField{Name: "name", Max: 48},
		&core.BoolField{Name: "selectable"},
	)
	if err := app.Save(plates); err != nil {
		t.Fatalf("save nameplates: %v", err)
	}

	app.OnRecordUpdate("users").BindFunc(syncGamertagFromDefault)
	return app
}

func newTestUser(t *testing.T, app core.App, email string) *core.Record {
	t.Helper()
	col, _ := app.FindCollectionByNameOrId("users")
	u := core.NewRecord(col)
	u.Set("email", email)
	u.Set("password", "0123456789")
	if err := app.Save(u); err != nil {
		t.Fatalf("save user: %v", err)
	}
	return u
}

func newTag(t *testing.T, app core.App, tag, status string) *core.Record {
	t.Helper()
	col, _ := app.FindCollectionByNameOrId("gamertags")
	r := core.NewRecord(col)
	r.Set("tag", tag)
	r.Set("status", status)
	if err := app.Save(r); err != nil {
		t.Fatalf("save tag: %v", err)
	}
	return r
}

func TestSyncGamertagFromDefault(t *testing.T) {
	app := streamIdentityApp(t)
	u := newTestUser(t, app, "sync@test.dev")
	approved := newTag(t, app, "StewGoal", "approved")
	blocked := newTag(t, app, "Naughty", "blocked")

	// Changing the default syncs users.gamertag to the tag's text.
	u.Set("default_gamertag", approved.Id)
	if err := app.Save(u); err != nil {
		t.Fatalf("set default: %v", err)
	}
	u, _ = app.FindRecordById("users", u.Id)
	if got := u.GetString("gamertag"); got != "StewGoal" {
		t.Fatalf("gamertag = %q, want StewGoal", got)
	}

	// A blocked default must not reach the generated saves.
	u.Set("default_gamertag", blocked.Id)
	if err := app.Save(u); err != nil {
		t.Fatalf("set blocked default: %v", err)
	}
	u, _ = app.FindRecordById("users", u.Id)
	if got := u.GetString("gamertag"); got != "StewGoal" {
		t.Fatalf("blocked default leaked: gamertag = %q", got)
	}

	// Clearing the default leaves the in-game name alone.
	u.Set("default_gamertag", "")
	if err := app.Save(u); err != nil {
		t.Fatalf("clear default: %v", err)
	}
	u, _ = app.FindRecordById("users", u.Id)
	if got := u.GetString("gamertag"); got != "StewGoal" {
		t.Fatalf("cleared default blanked gamertag: %q", got)
	}
}

func TestUsersNameplateGuard(t *testing.T) {
	app := streamIdentityApp(t)
	app.OnRecordUpdate("users").BindFunc(guardUserNameplate)

	platesCol, _ := app.FindCollectionByNameOrId("nameplates")
	selectable := core.NewRecord(platesCol)
	selectable.Set("name", "Neon")
	selectable.Set("selectable", true)
	if err := app.Save(selectable); err != nil {
		t.Fatalf("save plate: %v", err)
	}
	hidden := core.NewRecord(platesCol)
	hidden.Set("name", "Craig")
	hidden.Set("selectable", false)
	if err := app.Save(hidden); err != nil {
		t.Fatalf("save hidden plate: %v", err)
	}

	u := newTestUser(t, app, "plates@test.dev")

	// Picking a selectable banner works.
	u.Set("nameplate", selectable.Id)
	if err := app.Save(u); err != nil {
		t.Fatalf("pick selectable: %v", err)
	}

	// A hidden banner can't be newly picked…
	u, _ = app.FindRecordById("users", u.Id)
	u.Set("nameplate", hidden.Id)
	if err := app.Save(u); err == nil {
		t.Fatal("picking a hidden banner should fail")
	}

	// …but a worn banner survives the organizer hiding it (unrelated update).
	selectable.Set("selectable", false)
	if err := app.Save(selectable); err != nil {
		t.Fatalf("hide worn plate: %v", err)
	}
	u, _ = app.FindRecordById("users", u.Id)
	u.Set("email", "plates2@test.dev")
	if err := app.Save(u); err != nil {
		t.Fatalf("unrelated update with a now-hidden worn banner: %v", err)
	}

	// Clearing always works.
	u, _ = app.FindRecordById("users", u.Id)
	u.Set("nameplate", "")
	if err := app.Save(u); err != nil {
		t.Fatalf("clear: %v", err)
	}
}
