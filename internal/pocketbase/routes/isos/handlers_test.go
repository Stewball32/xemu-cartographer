package isos

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/Stewball32/xemu-cartographer/internal/authz/pb/pbtest"
)

// TestResolveServerISO covers the optional server_iso validation: "" clears the
// link, a self-reference is rejected, a non-existent id is rejected, and an
// existing catalog id is accepted.
func TestResolveServerISO(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)

	col := core.NewBaseCollection(collectionName)
	col.Fields.Add(&core.TextField{Name: "filename"})
	if err := app.Save(col); err != nil {
		t.Fatalf("save isos: %v", err)
	}
	existing := core.NewRecord(col)
	existing.Set("filename", "server.iso")
	if err := app.Save(existing); err != nil {
		t.Fatalf("save existing: %v", err)
	}

	t.Run("empty clears the link", func(t *testing.T) {
		id, msg := resolveServerISO(app, "   ", "self-id")
		if id != "" || msg != "" {
			t.Fatalf("got (%q,%q), want empty clear", id, msg)
		}
	})
	t.Run("self-reference rejected", func(t *testing.T) {
		if _, msg := resolveServerISO(app, existing.Id, existing.Id); msg == "" {
			t.Error("expected rejection for self-reference")
		}
	})
	t.Run("non-existent rejected", func(t *testing.T) {
		if _, msg := resolveServerISO(app, "no-such-id", "self-id"); msg == "" {
			t.Error("expected rejection for non-existent id")
		}
	})
	t.Run("existing accepted", func(t *testing.T) {
		id, msg := resolveServerISO(app, existing.Id, "self-id")
		if id != existing.Id || msg != "" {
			t.Fatalf("got (%q,%q), want (%q,\"\")", id, msg, existing.Id)
		}
	})
}

// newISOCollection adds a minimal `isos` collection (the columns handleUpdate
// writes) to a pbtest app and returns one catalog row.
func newISOCollection(t *testing.T, app core.App) *core.Record {
	t.Helper()
	col := core.NewBaseCollection(collectionName)
	col.Fields.Add(
		&core.TextField{Name: "name"},
		&core.TextField{Name: "description"},
		&core.SelectField{Name: "role", Values: []string{"play", "server", "shelved"}, MaxSelect: 1},
		&core.BoolField{Name: "allow_on_xbox"},
	)
	if err := app.Save(col); err != nil {
		t.Fatalf("save isos: %v", err)
	}
	rec := core.NewRecord(col)
	rec.Set("name", "Halo")
	rec.Set("role", "play")
	if err := app.Save(rec); err != nil {
		t.Fatalf("save iso: %v", err)
	}
	return rec
}

// patch runs handleUpdate over a synthetic PATCH /api/admin/isos/{id} event
// and returns the status. apis.ApiError returns fold into their status code.
func patch(t *testing.T, app core.App, auth *core.Record, id, body string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/api/admin/isos/"+id, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	e := &core.RequestEvent{
		App:   app,
		Auth:  auth,
		Event: router.Event{Request: req, Response: rec},
	}
	if err := handleUpdate(e); err != nil {
		var apiErr *router.ApiError
		if errors.As(err, &apiErr) {
			return apiErr.Status
		}
		t.Fatalf("handleUpdate returned a non-API error: %v", err)
	}
	if rec.Code == http.StatusOK {
		var out map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("200 body %q: %v", rec.Body.String(), err)
		}
	}
	return rec.Code
}

// TestPatchPolicyFieldsRequireSetPolicy pins R-3: metadata edits need only the
// group's library.manage, while the policy fields (role / allow_on_xbox)
// additionally require iso.set_policy on the ISO — seeded on organizer, so
// the default is no behaviour change, and a 403 with nothing written once the
// scope is removed from the row.
func TestPatchPolicyFieldsRequireSetPolicy(t *testing.T) {
	app, _ := pbtest.NewApp(t)
	iso := newISOCollection(t, app)

	organizer := pbtest.NewUser(t, app, "org@test.dev")
	pbtest.GrantRole(t, app, organizer.Id, "organizer")
	admin := pbtest.NewUser(t, app, "admin@test.dev")
	pbtest.GrantRole(t, app, admin.Id, "admin")
	member := pbtest.NewUser(t, app, "member@test.dev")
	pbtest.GrantRole(t, app, member.Id, "member")
	superuser := pbtest.NewSuperuser(t, app, "root@test.dev")

	reload := func() *core.Record {
		rec, err := app.FindRecordById(collectionName, iso.Id)
		if err != nil {
			t.Fatalf("reload iso: %v", err)
		}
		return rec
	}

	t.Run("organizer metadata edit", func(t *testing.T) {
		if code := patch(t, app, organizer, iso.Id, `{"name":"Halo CE"}`); code != http.StatusOK {
			t.Fatalf("organizer {name}: %d", code)
		}
		if got := reload().GetString("name"); got != "Halo CE" {
			t.Fatalf("name = %q, want Halo CE", got)
		}
	})

	t.Run("organizer policy edit with the seeded scope", func(t *testing.T) {
		if code := patch(t, app, organizer, iso.Id, `{"allow_on_xbox":true}`); code != http.StatusOK {
			t.Fatalf("organizer {allow_on_xbox} seeded: %d", code)
		}
		if !reload().GetBool("allow_on_xbox") {
			t.Fatal("allow_on_xbox not written")
		}
		if code := patch(t, app, organizer, iso.Id, `{"role":"server"}`); code != http.StatusOK {
			t.Fatalf("organizer {role} seeded: %d", code)
		}
		if got := reload().GetString("role"); got != "server" {
			t.Fatalf("role = %q, want server", got)
		}
	})

	// Strip iso.set_policy from the organizer row: policy edits become 403
	// and — because the check runs before any rec.Set — nothing in the same
	// body is written either; plain metadata edits still pass.
	pbtest.SetRoleScopes(t, app, "organizer", []string{"library.manage"})

	t.Run("organizer policy edit without the scope", func(t *testing.T) {
		before := reload()
		if code := patch(t, app, organizer, iso.Id, `{"name":"Renamed","allow_on_xbox":false}`); code != http.StatusForbidden {
			t.Fatalf("organizer {allow_on_xbox} stripped: %d, want 403", code)
		}
		if code := patch(t, app, organizer, iso.Id, `{"role":"shelved"}`); code != http.StatusForbidden {
			t.Fatalf("organizer {role} stripped: %d, want 403", code)
		}
		after := reload()
		if after.GetString("name") != before.GetString("name") ||
			after.GetBool("allow_on_xbox") != before.GetBool("allow_on_xbox") ||
			after.GetString("role") != before.GetString("role") {
			t.Fatalf("denied PATCH mutated the record: before=%v after=%v", before.PublicExport(), after.PublicExport())
		}
		if code := patch(t, app, organizer, iso.Id, `{"name":"Still Editable"}`); code != http.StatusOK {
			t.Fatalf("organizer {name} stripped: %d, want 200", code)
		}
	})

	t.Run("admin and superuser keep policy edits", func(t *testing.T) {
		if code := patch(t, app, admin, iso.Id, `{"allow_on_xbox":false}`); code != http.StatusOK {
			t.Fatalf("admin {allow_on_xbox}: %d", code)
		}
		if code := patch(t, app, superuser, iso.Id, `{"role":"play"}`); code != http.StatusOK {
			t.Fatalf("superuser {role}: %d", code)
		}
	})

	t.Run("member and anonymous are refused at the handler too", func(t *testing.T) {
		if code := patch(t, app, member, iso.Id, `{"allow_on_xbox":true}`); code != http.StatusForbidden {
			t.Fatalf("member {allow_on_xbox}: %d, want 403", code)
		}
		if code := patch(t, app, nil, iso.Id, `{"allow_on_xbox":true}`); code != http.StatusUnauthorized {
			t.Fatalf("anonymous {allow_on_xbox}: %d, want 401", code)
		}
	})
}
