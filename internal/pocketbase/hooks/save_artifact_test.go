package hooks

import (
	"archive/tar"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

// This is the integration test for the generate-on-save path — the one piece
// that can't be unit-tested in isolation: a record hook that generates bytes
// and attaches them to a PocketBase file field, in the same save transaction,
// then reads them back. Uses a real test PB app.

func ensureProfileCollections(t *testing.T, app core.App) {
	t.Helper()
	usersCol, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatalf("users collection: %v", err)
	}
	// gamertag is the single source of truth on the user record (cap 11 = CE's
	// MP name limit; all test gamertags are short).
	if usersCol.Fields.GetByName("gamertag") == nil {
		usersCol.Fields.Add(&core.TextField{Name: "gamertag", Max: 11})
		if err := app.Save(usersCol); err != nil {
			t.Fatalf("add users.gamertag: %v", err)
		}
	}

	if _, err := app.FindCollectionByNameOrId("h2_profiles"); err != nil {
		c := core.NewBaseCollection("h2_profiles")
		c.Fields.Add(
			&core.RelationField{Name: "user", Required: true, CollectionId: usersCol.Id, MaxSelect: 1},
			&core.JSONField{Name: "appearance", MaxSize: 1 << 16},
			&core.FileField{Name: "save_bundle", MaxSelect: 1, MaxSize: 1 << 20},
			&core.JSONField{Name: "save_info", MaxSize: 1 << 16},
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)
		if err := app.Save(c); err != nil {
			t.Fatalf("save h2_profiles: %v", err)
		}
	}

	if _, err := app.FindCollectionByNameOrId("ce_profiles"); err != nil {
		c := core.NewBaseCollection("ce_profiles")
		c.Fields.Add(
			&core.RelationField{Name: "user", Required: true, CollectionId: usersCol.Id, MaxSelect: 1},
			&core.JSONField{Name: "settings", MaxSize: 1 << 16},
			&core.FileField{Name: "save_bundle", MaxSelect: 1, MaxSize: 1 << 20},
			&core.JSONField{Name: "save_info", MaxSize: 1 << 16},
			&core.AutodateField{Name: "created", OnCreate: true},
			&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true},
		)
		if err := app.Save(c); err != nil {
			t.Fatalf("save ce_profiles: %v", err)
		}
	}

	if _, err := app.FindCollectionByNameOrId("gametypes"); err != nil {
		c := core.NewBaseCollection("gametypes")
		c.Fields.Add(
			&core.SelectField{Name: "title", Required: true, MaxSelect: 1, Values: []string{"ce", "h2"}},
			&core.TextField{Name: "engine", Required: true, Max: 32},
			&core.TextField{Name: "name", Required: true, Max: 64},
			&core.JSONField{Name: "settings", MaxSize: 1 << 16},
			&core.FileField{Name: "save_bundle", MaxSelect: 1, MaxSize: 1 << 20},
			&core.JSONField{Name: "save_info", MaxSize: 1 << 16},
			&core.AutodateField{Name: "created", OnCreate: true},
		)
		if err := app.Save(c); err != nil {
			t.Fatalf("save gametypes: %v", err)
		}
	}
}

func makeUser(t *testing.T, app core.App, gamertag string) *core.Record {
	t.Helper()
	col, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		t.Fatalf("users: %v", err)
	}
	u := core.NewRecord(col)
	u.Set("email", "player@dev.local")
	u.Set("password", "password1234")
	u.Set("gamertag", gamertag)
	if err := app.Save(u); err != nil {
		t.Fatalf("save user: %v", err)
	}
	return u
}

// readBundle reads a record's save_bundle file back through the app filesystem.
func readBundle(t *testing.T, app core.App, rec *core.Record) []byte {
	t.Helper()
	filename := rec.GetString("save_bundle")
	if filename == "" {
		t.Fatal("save_bundle is empty — the hook did not attach a file")
	}
	fsys, err := app.NewFilesystem()
	if err != nil {
		t.Fatalf("NewFilesystem: %v", err)
	}
	defer fsys.Close()
	r, err := fsys.GetReader(rec.BaseFilesPath() + "/" + filename)
	if err != nil {
		t.Fatalf("GetReader: %v", err)
	}
	defer r.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read bundle: %v", err)
	}
	return data
}

func tarNames(t *testing.T, blob []byte) []string {
	t.Helper()
	var names []string
	tr := tar.NewReader(bytes.NewReader(blob))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar: %v", err)
		}
		names = append(names, hdr.Name)
	}
	return names
}

func TestH2ProfileGenerateOnSave(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureProfileCollections(t, app)
	app.OnRecordCreate("h2_profiles").BindFunc(generateH2Profile)
	app.OnRecordUpdate("h2_profiles").BindFunc(generateH2Profile)
	// Changing users.gamertag must cascade-regenerate the profiles.
	app.OnRecordUpdate("users").BindFunc(regenerateProfilesOnGamertagChange)

	user := makeUser(t, app, "CARTOG")
	col, _ := app.FindCollectionByNameOrId("h2_profiles")
	rec := core.NewRecord(col)
	rec.Set("user", user.Id) // gamertag comes from the user, not the profile
	rec.Set("appearance", map[string]int{"armor_primary": 13})
	if err := app.Save(rec); err != nil {
		t.Fatalf("save h2_profiles: %v", err)
	}

	// The hook should have attached a tar bundle + non-deferred save_info.
	bundle := readBundle(t, app, rec)
	names := tarNames(t, bundle)
	var hasProfile, hasMeta bool
	for _, n := range names {
		if strings.HasSuffix(n, "/profile") {
			hasProfile = true
		}
		if strings.HasSuffix(n, "/SaveMeta.xbx") {
			hasMeta = true
		}
	}
	if !hasProfile || !hasMeta {
		t.Fatalf("bundle missing profile/SaveMeta; entries = %v", names)
	}
	if info := rec.GetString("save_info"); !strings.Contains(info, "4d530064") {
		t.Errorf("save_info missing H2 title id; got %q", info)
	}

	// Renaming the user's gamertag must cascade-regenerate the profile (the
	// in-game name lives in the file), without touching the profile directly.
	user.Set("gamertag", "RENAMED")
	if err := app.Save(user); err != nil {
		t.Fatalf("update user gamertag: %v", err)
	}
	fresh, err := app.FindRecordById("h2_profiles", rec.Id)
	if err != nil {
		t.Fatalf("refetch profile: %v", err)
	}
	bundle2 := readBundle(t, app, fresh)
	if bytes.Equal(bundle, bundle2) {
		t.Error("expected the regenerated bundle to differ after a gamertag rename")
	}
}

func TestH2ProfileRejectsBadAppearance(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureProfileCollections(t, app)
	app.OnRecordCreate("h2_profiles").BindFunc(generateH2Profile)

	user := makeUser(t, app, "X")
	col, _ := app.FindCollectionByNameOrId("h2_profiles")
	rec := core.NewRecord(col)
	rec.Set("user", user.Id)
	rec.Set("appearance", map[string]int{"armor_primary": 999}) // out of byte range
	if err := app.Save(rec); err == nil {
		t.Fatal("expected save to fail when generation rejects the appearance byte")
	}
}

func TestGametypeGenerateOnSave(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureProfileCollections(t, app)
	app.OnRecordCreate("gametypes").BindFunc(generateGametype)

	col, _ := app.FindCollectionByNameOrId("gametypes")
	rec := core.NewRecord(col)
	rec.Set("title", "ce")
	rec.Set("engine", "slayer")
	rec.Set("name", "TS 50")
	rec.Set("settings", map[string]any{"score_limit": 50, "teams": true})
	if err := app.Save(rec); err != nil {
		t.Fatalf("save gametypes: %v", err)
	}

	bundle := readBundle(t, app, rec)
	names := tarNames(t, bundle)
	var hasBlam bool
	for _, n := range names {
		if strings.HasSuffix(n, "/blam.lst") {
			hasBlam = true
		}
	}
	if !hasBlam {
		t.Fatalf("CE gametype bundle missing blam.lst; entries = %v", names)
	}
}

func TestCeProfileGeneratesBlamSav(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)
	ensureProfileCollections(t, app)
	app.OnRecordCreate("ce_profiles").BindFunc(generateCeProfile)

	user := makeUser(t, app, "CARTOG")
	col, _ := app.FindCollectionByNameOrId("ce_profiles")
	rec := core.NewRecord(col)
	rec.Set("user", user.Id)
	rec.Set("settings", map[string]any{"color": 2, "thumbstick": 1, "button": 0})
	if err := app.Save(rec); err != nil {
		t.Fatalf("save ce_profiles: %v", err)
	}

	bundle := readBundle(t, app, rec)
	names := tarNames(t, bundle)
	var hasBlam bool
	for _, n := range names {
		if strings.HasSuffix(n, "/blam.sav") {
			hasBlam = true
		}
	}
	if !hasBlam {
		t.Fatalf("CE profile bundle missing blam.sav; entries = %v", names)
	}
	if info := rec.GetString("save_info"); !strings.Contains(info, "4d530004") {
		t.Errorf("save_info missing CE title id; got %q", info)
	}
}
