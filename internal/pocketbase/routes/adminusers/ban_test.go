package adminusers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/pb/pbtest"
)

// call drives one ban-surface handler as auth against a synthetic
// /api/admin/users/{id}/… request (the group middleware is not in play — it
// only proves admin.users) and returns the status: an apis error's Status
// or the recorder's code, with the decoded JSON body.
func call(t *testing.T, app core.App, auth *core.Record, targetID, verb, body string, h func(*core.RequestEvent) error) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/"+targetID+"/"+verb, strings.NewReader(body))
	req.SetPathValue("id", targetID)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	e := &core.RequestEvent{
		App:   app,
		Auth:  auth,
		Event: router.Event{Request: req, Response: rec},
	}
	err := h(e)
	if err != nil {
		var apiErr *router.ApiError
		if errors.As(err, &apiErr) {
			return apiErr.Status, map[string]any{"message": apiErr.Message}
		}
		t.Fatalf("handler returned a non-API error: %v", err)
	}
	out := map[string]any{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("body %q: %v", rec.Body.String(), err)
		}
	}
	return rec.Code, out
}

// TestBanRoutesRequireUserModerate pins that ban / unban / timeout check
// user.moderate on the target (design §2.3, §4) beyond the group's
// admin.users gate: an admin (user.* in the seed) and a superuser mutate the
// ban state; a role that reaches /api/admin/users with admin.users alone is
// refused with 403 and the target is left untouched.
func TestBanRoutesRequireUserModerate(t *testing.T) {
	app, _ := pbtest.NewApp(t)

	admin := pbtest.NewUser(t, app, "admin@test.dev")
	pbtest.GrantRole(t, app, admin.Id, "admin")
	super := pbtest.NewSuperuser(t, app, "root@test.dev")
	// An organizer whose role reaches the group (admin.users) but carries no
	// user.moderate.
	lister := pbtest.NewUser(t, app, "lister@test.dev")
	pbtest.GrantRole(t, app, lister.Id, "organizer")
	pbtest.SetRoleScopes(t, app, "organizer", []string{"admin.users"})
	target := pbtest.NewUser(t, app, "target@test.dev")

	banned := func() bool {
		rec, err := app.FindRecordById("users", target.Id)
		if err != nil {
			t.Fatalf("reload target: %v", err)
		}
		return rec.GetBool("is_banned")
	}

	// admin.users alone: every verb is a 403 and nothing changes.
	for _, c := range []struct {
		verb string
		body string
		h    func(*core.RequestEvent) error
	}{
		{"ban", `{"reason":"spam"}`, handleBan},
		{"timeout", `{"expires_at":"2099-01-01T00:00:00.000Z"}`, handleTimeout},
		{"unban", ``, handleUnban},
	} {
		code, body := call(t, app, lister, target.Id, c.verb, c.body, c.h)
		if code != http.StatusForbidden {
			t.Errorf("%s as admin.users-only: status = %d body=%v, want 403", c.verb, code, body)
		}
	}
	if banned() {
		t.Fatal("a refused ban must not change the target")
	}

	// Admin: ban → unban → timeout all succeed and move the state.
	if code, body := call(t, app, admin, target.Id, "ban", `{"reason":"spam"}`, handleBan); code != http.StatusOK || body["is_banned"] != true {
		t.Fatalf("admin ban: status = %d body=%v", code, body)
	}
	if !banned() {
		t.Fatal("admin ban did not set is_banned")
	}
	if code, body := call(t, app, admin, target.Id, "unban", ``, handleUnban); code != http.StatusOK || body["is_banned"] != false {
		t.Fatalf("admin unban: status = %d body=%v", code, body)
	}
	if banned() {
		t.Fatal("admin unban did not clear is_banned")
	}
	if code, body := call(t, app, admin, target.Id, "timeout", `{"expires_at":"2099-01-01T00:00:00.000Z"}`, handleTimeout); code != http.StatusOK || body["is_banned"] != true {
		t.Fatalf("admin timeout: status = %d body=%v", code, body)
	}
	if !banned() {
		t.Fatal("admin timeout did not set is_banned")
	}

	// Superuser: passes the check like everything else.
	if code, body := call(t, app, super, target.Id, "unban", ``, handleUnban); code != http.StatusOK {
		t.Fatalf("superuser unban: status = %d body=%v", code, body)
	}
	if banned() {
		t.Fatal("superuser unban did not clear is_banned")
	}

	// Unknown target is still the 404 (the check runs on the loaded user).
	if code, _ := call(t, app, admin, "nope", "ban", ``, handleBan); code != http.StatusNotFound {
		t.Errorf("unknown target: status = %d, want 404", code)
	}
	// Self-ban stays the 400 for a caller who may moderate.
	if code, _ := call(t, app, admin, admin.Id, "ban", ``, handleBan); code != http.StatusBadRequest {
		t.Errorf("self ban: status = %d, want 400", code)
	}
}
