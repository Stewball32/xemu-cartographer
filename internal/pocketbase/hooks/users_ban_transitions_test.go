package hooks

import (
	"errors"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/Stewball32/xemu-cartographer/internal/audit"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb/pbtest"
)

// Integration tests for the H-1..H-4 authz gates. Each hook is a named
// OnRecordUpdateRequest handler; the tests build the request event by hand
// (Next() on a bare hook.Event is a no-op) so a call returns the hook's own
// verdict. pbtest.NewApp installs pb.Default(), which the handlers resolve
// at request time.

// authzRequestEvent builds an OnRecordUpdateRequest event for rec acting
// as auth (nil = in-process writer).
func authzRequestEvent(app core.App, auth, rec *core.Record) *core.RecordRequestEvent {
	e := &core.RecordRequestEvent{
		RequestEvent: &core.RequestEvent{App: app, Auth: auth},
		Record:       rec,
	}
	e.Collection = rec.Collection()
	return e
}

// reload re-reads a record so Original() reflects the stored row.
func reload(t *testing.T, app core.App, rec *core.Record) *core.Record {
	t.Helper()
	fresh, err := app.FindRecordById(rec.Collection().Name, rec.Id)
	if err != nil {
		t.Fatalf("reload %s/%s: %v", rec.Collection().Name, rec.Id, err)
	}
	return fresh
}

// memberUser creates a plain member (level 10, no scopes).
func memberUser(t *testing.T, app core.App, email string) *core.Record {
	t.Helper()
	u := pbtest.NewUser(t, app, email)
	pbtest.GrantRole(t, app, u.Id, "member")
	return u
}

// adminUser creates a user carrying the admin role.
func adminUser(t *testing.T, app core.App, email string) *core.Record {
	t.Helper()
	u := pbtest.NewUser(t, app, email)
	pbtest.GrantRole(t, app, u.Id, "admin")
	return u
}

// wantAPIError asserts err is a PocketBase API error with the given status.
func wantAPIError(t *testing.T, err error, status int) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected a %d API error, got nil", status)
	}
	var apiErr *router.ApiError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *router.ApiError, got %T: %v", err, err)
	}
	if apiErr.Status != status {
		t.Fatalf("status = %d, want %d (%v)", apiErr.Status, status, err)
	}
}

// countAudit returns the number of audit_log rows with the given action.
func countAudit(t *testing.T, app core.App, action audit.Action) int {
	t.Helper()
	rows, err := app.FindRecordsByFilter("audit_log", "action = {:a}", "", 0, 0, map[string]any{"a": string(action)})
	if err != nil {
		t.Fatalf("audit_log query: %v", err)
	}
	return len(rows)
}

func TestUsersBanTransitions_InternalActorAllowed(t *testing.T) {
	app, _ := pbtest.NewApp(t)
	target := reload(t, app, pbtest.NewUser(t, app, "target@test.dev"))
	target.Set("is_banned", true)

	if err := usersBanTransitions(authzRequestEvent(app, nil, target)); err != nil {
		t.Fatalf("internal actor: %v", err)
	}
	if got := countAudit(t, app, audit.ActionBan); got != 1 {
		t.Fatalf("ActionBan audit rows = %d, want 1", got)
	}
}

func TestUsersBanTransitions_MemberDenied(t *testing.T) {
	app, _ := pbtest.NewApp(t)
	member := memberUser(t, app, "member@test.dev")
	target := reload(t, app, pbtest.NewUser(t, app, "target@test.dev"))
	target.Set("is_banned", true)

	err := usersBanTransitions(authzRequestEvent(app, member, target))
	wantAPIError(t, err, 403)
	if got := countAudit(t, app, audit.ActionBan); got != 0 {
		t.Fatalf("ActionBan audit rows = %d, want 0", got)
	}

	// A member editing their own row can't clear a ban either.
	pbtest.SetField(t, app, "users", member.Id, "is_banned", true)
	self := reload(t, app, member)
	self.Set("is_banned", false)
	wantAPIError(t, usersBanTransitions(authzRequestEvent(app, member, self)), 403)
}

func TestUsersBanTransitions_AdminAllowed(t *testing.T) {
	app, _ := pbtest.NewApp(t)
	admin := adminUser(t, app, "admin@test.dev")
	target := reload(t, app, pbtest.NewUser(t, app, "target@test.dev"))
	target.Set("is_banned", true)

	if err := usersBanTransitions(authzRequestEvent(app, admin, target)); err != nil {
		t.Fatalf("admin: %v", err)
	}
	if got := countAudit(t, app, audit.ActionBan); got != 1 {
		t.Fatalf("ActionBan audit rows = %d, want 1", got)
	}
}

func TestUsersBanTransitions_NoDepsDenied(t *testing.T) {
	app, _ := pbtest.NewApp(t)
	admin := adminUser(t, app, "admin@test.dev")
	target := reload(t, app, pbtest.NewUser(t, app, "target@test.dev"))
	target.Set("is_banned", true)

	// Before boot installs the adapter the gate fails closed, even for an
	// admin and even for the in-process writer.
	pb.SetDefault(nil)
	wantAPIError(t, usersBanTransitions(authzRequestEvent(app, admin, target)), 403)
	wantAPIError(t, usersBanTransitions(authzRequestEvent(app, nil, target)), 403)

	// An untouched update is not a moderation write and still passes.
	plain := reload(t, app, target)
	plain.Set("name", "renamed")
	if err := usersBanTransitions(authzRequestEvent(app, admin, plain)); err != nil {
		t.Fatalf("non-ban update with no deps: %v", err)
	}
}
