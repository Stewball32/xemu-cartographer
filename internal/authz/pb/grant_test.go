package pb_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/audit"
	"github.com/Stewball32/xemu-cartographer/internal/authz"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb/pbtest"
)

// userRole finds the user_roles row for (user, slug), nil when absent.
func userRole(t *testing.T, app core.App, userID, slug string) *core.Record {
	t.Helper()
	rows, err := app.FindRecordsByFilter("user_roles", "user = {:u} && role.slug = {:s}", "", 1, 0,
		dbx.Params{"u": userID, "s": slug})
	if err != nil {
		t.Fatalf("user_roles lookup: %v", err)
	}
	if len(rows) == 0 {
		return nil
	}
	return rows[0]
}

// auditRows returns the audit_log rows for an action, oldest first.
func auditRows(t *testing.T, app core.App, action audit.Action) []*core.Record {
	t.Helper()
	rows, err := app.FindRecordsByFilter("audit_log", "action = {:a}", "created", 0, 0, dbx.Params{"a": string(action)})
	if err != nil {
		t.Fatalf("audit_log lookup: %v", err)
	}
	return rows
}

func payloadOf(t *testing.T, row *core.Record, into any) {
	t.Helper()
	if err := json.Unmarshal([]byte(row.GetString("payload_json")), into); err != nil {
		t.Fatalf("payload_json %q: %v", row.GetString("payload_json"), err)
	}
}

func TestGrantTiering(t *testing.T) {
	app, d := pbtest.NewApp(t)
	// A level-50 actor holding role.* (the organizer seed has no role scope;
	// widen it so the level tier — not the scope — is what decides).
	pbtest.SetRoleScopes(t, app, "organizer", []string{"role.*", "library.manage:*"})
	actorRec := pbtest.NewUser(t, app, "org@test.dev")
	pbtest.GrantRole(t, app, actorRec.Id, "organizer")
	actor := pb.PrincipalFromAuth(app, d, actorRec)
	if actor.Level != 50 {
		t.Fatalf("actor level = %d", actor.Level)
	}
	target := pbtest.NewUser(t, app, "target@test.dev")

	err := pb.Grant(app, d, actor, target.Id, "admin", "promote")
	if !errors.Is(err, authz.ErrLevelTooLow) {
		t.Fatalf("level 50 granting admin: %v", err)
	}
	if userRole(t, app, target.Id, "admin") != nil {
		t.Fatalf("admin row written despite refusal")
	}
	if err := pb.Grant(app, d, actor, target.Id, "wizard", ""); !errors.Is(err, authz.ErrUnknownRole) {
		t.Fatalf("unknown role: %v", err)
	}

	if err := pb.Grant(app, d, actor, target.Id, "member", "welcome"); err != nil {
		t.Fatalf("level 50 granting member: %v", err)
	}
	row := userRole(t, app, target.Id, "member")
	if row == nil {
		t.Fatalf("member row missing")
	}
	if got := row.GetString("granted_by"); got != actorRec.Id {
		t.Fatalf("granted_by = %q, want %q", got, actorRec.Id)
	}
	// Idempotent.
	if err := pb.Grant(app, d, actor, target.Id, "member", ""); err != nil {
		t.Fatalf("second grant: %v", err)
	}
	rows := auditRows(t, app, audit.ActionRoleGrant)
	if len(rows) != 1 {
		t.Fatalf("audit rows = %d, want 1", len(rows))
	}
	if rows[0].GetString("actor") != actorRec.Id || rows[0].GetString("target_id") != target.Id {
		t.Fatalf("audit actor/target = %q/%q", rows[0].GetString("actor"), rows[0].GetString("target_id"))
	}
	var payload audit.RoleGrantPayload
	payloadOf(t, rows[0], &payload)
	if payload.RoleSlug != "member" || payload.Reason != "welcome" || payload.BySuperuser || payload.ByMigration {
		t.Fatalf("payload = %+v", payload)
	}

	// The granted user now carries the role on the next resolve.
	if p := pb.PrincipalFromAuth(app, d, target); !p.HasRole("member") {
		t.Fatalf("target roles = %v", p.Roles)
	}

	// An actor without the role scope is refused with ErrForbidden.
	plain := pbtest.NewUser(t, app, "plain@test.dev")
	pbtest.GrantRole(t, app, plain.Id, "member")
	err = pb.Grant(app, d, pb.PrincipalFromAuth(app, d, plain), target.Id, "member", "")
	if !errors.Is(err, pb.ErrForbidden) {
		t.Fatalf("scopeless actor: %v", err)
	}
	// An admin (100) may grant admin.
	adminRec := pbtest.NewUser(t, app, "admin@test.dev")
	pbtest.GrantRole(t, app, adminRec.Id, "admin")
	if err := pb.Grant(app, d, pb.PrincipalFromAuth(app, d, adminRec), target.Id, "admin", ""); err != nil {
		t.Fatalf("admin granting admin: %v", err)
	}
}

func TestRevokeLastAdmin(t *testing.T) {
	app, d := pbtest.NewApp(t)
	only := pbtest.NewUser(t, app, "only@test.dev")
	pbtest.GrantRole(t, app, only.Id, "admin")
	if n := d.AdminCount(); n != 1 {
		t.Fatalf("AdminCount = %d", n)
	}

	su := authz.Superuser("root")
	// The deny is about the holder: revoking admin from a user who does not
	// hold it is the usual no-op, not a 409, even with one admin left.
	bystander := pbtest.NewUser(t, app, "bystander@test.dev")
	if err := pb.Revoke(app, d, su, bystander.Id, "admin", ""); err != nil {
		t.Fatalf("no-op revoke of a non-holder with one admin: %v", err)
	}
	err := pb.Revoke(app, d, su, only.Id, "admin", "oops")
	if !errors.Is(err, authz.ErrLastAdmin) {
		t.Fatalf("superuser revoking the last admin: %v", err)
	}
	if userRole(t, app, only.Id, "admin") == nil {
		t.Fatalf("last admin row deleted despite refusal")
	}

	// With a second admin the superuser may revoke one.
	second := pbtest.NewUser(t, app, "second@test.dev")
	pbtest.GrantRole(t, app, second.Id, "admin")
	if err := pb.Revoke(app, d, su, second.Id, "admin", "rotate"); err != nil {
		t.Fatalf("superuser revoking one of two: %v", err)
	}
	if userRole(t, app, second.Id, "admin") != nil {
		t.Fatalf("second admin row still present")
	}
	rows := auditRows(t, app, audit.ActionRoleRevoke)
	if len(rows) != 1 || rows[0].GetString("actor") != "" {
		t.Fatalf("revoke audit rows = %d (actor %q)", len(rows), rows[0].GetString("actor"))
	}
	var payload audit.RoleRevokePayload
	payloadOf(t, rows[0], &payload)
	if !payload.BySuperuser || payload.RoleSlug != "admin" || payload.Reason != "rotate" {
		t.Fatalf("payload = %+v", payload)
	}

	// The internal cascade may remove the last admin, at WARN.
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	if err := pb.Revoke(app, d, authz.Internal("users_soft_delete"), only.Id, "admin", "soft delete"); err != nil {
		t.Fatalf("internal revoke of the last admin: %v", err)
	}
	if userRole(t, app, only.Id, "admin") != nil {
		t.Fatalf("internal revoke did not delete the row")
	}
	if !strings.Contains(buf.String(), "last admin revoked by internal") {
		t.Fatalf("WARN line missing, log = %q", buf.String())
	}
	if n := d.AdminCount(); n != 0 {
		t.Fatalf("AdminCount after cascade = %d", n)
	}
	rows = auditRows(t, app, audit.ActionRoleRevoke)
	if len(rows) != 2 {
		t.Fatalf("revoke audit rows = %d, want 2", len(rows))
	}
	var internalPayload audit.RoleRevokePayload
	payloadOf(t, rows[1], &internalPayload)
	if internalPayload.Actor != "internal:users_soft_delete" || internalPayload.BySuperuser {
		t.Fatalf("internal payload = %+v", internalPayload)
	}

	// Revoking a role the user does not hold is a silent no-op.
	if err := pb.Revoke(app, d, su, only.Id, "member", ""); err != nil {
		t.Fatalf("no-op revoke: %v", err)
	}
	// A pb_user below the role's level may not revoke it either.
	low := pbtest.NewUser(t, app, "low@test.dev")
	pbtest.SetRoleScopes(t, app, "organizer", []string{"role.*"})
	pbtest.GrantRole(t, app, low.Id, "organizer")
	pbtest.GrantRole(t, app, second.Id, "admin")
	pbtest.GrantRole(t, app, only.Id, "admin")
	err = pb.Revoke(app, d, pb.PrincipalFromAuth(app, d, low), second.Id, "admin", "")
	if !errors.Is(err, authz.ErrLevelTooLow) {
		t.Fatalf("level 50 revoking admin: %v", err)
	}
}

func TestRevokeLastAdminRace(t *testing.T) {
	app, d := pbtest.NewApp(t)
	a := pbtest.NewUser(t, app, "a@test.dev")
	b := pbtest.NewUser(t, app, "b@test.dev")
	pbtest.GrantRole(t, app, a.Id, "admin")
	pbtest.GrantRole(t, app, b.Id, "admin")
	if n := d.AdminCount(); n != 2 {
		t.Fatalf("AdminCount = %d", n)
	}

	// Barrier on the delete: a revoke that reaches it has already passed the
	// last_admin decision. Without the transaction both revokes get here
	// holding AdminCount() == 2, the barrier releases them together and both
	// rows go. With it the second revoke's decision waits for the first
	// one's commit and counts one admin.
	var mu sync.Mutex
	arrived := 0
	gate := make(chan struct{})
	app.OnRecordDelete("user_roles").BindFunc(func(e *core.RecordEvent) error {
		mu.Lock()
		arrived++
		if arrived == 2 {
			close(gate)
		}
		mu.Unlock()
		select {
		case <-gate:
		case <-time.After(300 * time.Millisecond):
		}
		return e.Next()
	})

	su := authz.Superuser("root")
	errs := make(chan error, 2)
	for _, id := range []string{a.Id, b.Id} {
		go func(id string) { errs <- pb.Revoke(app, d, su, id, "admin", "race") }(id)
	}
	var passed, refused int
	for i := 0; i < 2; i++ {
		switch err := <-errs; {
		case err == nil:
			passed++
		case errors.Is(err, authz.ErrLastAdmin):
			refused++
		default:
			t.Fatalf("revoke: %v", err)
		}
	}
	if passed != 1 || refused != 1 {
		t.Fatalf("passed=%d refused=%d, want one of each", passed, refused)
	}
	if n := d.AdminCount(); n != 1 {
		t.Fatalf("AdminCount after the race = %d, want 1", n)
	}
	left := 0
	for _, id := range []string{a.Id, b.Id} {
		if userRole(t, app, id, "admin") != nil {
			left++
		}
	}
	if left != 1 {
		t.Fatalf("admin rows left = %d, want 1", left)
	}
	if rows := auditRows(t, app, audit.ActionRoleRevoke); len(rows) != 1 {
		t.Fatalf("revoke audit rows = %d, want 1", len(rows))
	}
}

func TestGrantAnonymousRefused(t *testing.T) {
	app, d := pbtest.NewApp(t)
	target := pbtest.NewUser(t, app, "t@test.dev")
	adminRec := pbtest.NewUser(t, app, "admin@test.dev")
	pbtest.GrantRole(t, app, adminRec.Id, "admin")
	if _, ok := d.RoleLevel("anonymous"); !ok {
		t.Fatalf("anonymous role not seeded")
	}

	for name, actor := range map[string]authz.Principal{
		"superuser": authz.Superuser("root"),
		"internal":  authz.Internal("seed"),
		"admin":     pb.PrincipalFromAuth(app, d, adminRec),
	} {
		for _, slug := range []string{"anonymous", " anonymous "} {
			err := pb.Grant(app, d, actor, target.Id, slug, "door")
			if !errors.Is(err, pb.ErrRoleNotGrantable) {
				t.Fatalf("%s granting %q: %v", name, slug, err)
			}
		}
	}
	if userRole(t, app, target.Id, "anonymous") != nil {
		t.Fatalf("anonymous user_roles row written")
	}
	if rows := auditRows(t, app, audit.ActionRoleGrant); len(rows) != 0 {
		t.Fatalf("grant audit rows = %d, want 0", len(rows))
	}
	if p := pb.PrincipalFromAuth(app, d, target); p.HasRole("anonymous") || len(p.Scopes) != 0 {
		t.Fatalf("target principal = %+v", p)
	}
}

func TestGrantSuperuserNoGrantedBy(t *testing.T) {
	app, d := pbtest.NewApp(t)
	suRec := pbtest.NewSuperuser(t, app, "root@test.dev")
	target := pbtest.NewUser(t, app, "t@test.dev")

	actor := pb.PrincipalFromAuth(app, d, suRec)
	if actor.Kind != authz.KindSuperuser {
		t.Fatalf("actor = %+v", actor)
	}
	if err := pb.Grant(app, d, actor, target.Id, "admin", "bootstrap"); err != nil {
		t.Fatalf("superuser grant: %v", err)
	}
	row := userRole(t, app, target.Id, "admin")
	if row == nil {
		t.Fatalf("admin row missing")
	}
	if got := row.GetString("granted_by"); got != "" {
		t.Fatalf("granted_by = %q, want empty for a superuser", got)
	}
	rows := auditRows(t, app, audit.ActionRoleGrant)
	if len(rows) != 1 {
		t.Fatalf("audit rows = %d", len(rows))
	}
	if rows[0].GetString("actor") != "" {
		t.Fatalf("audit actor = %q, want null for a superuser", rows[0].GetString("actor"))
	}
	var payload audit.RoleGrantPayload
	payloadOf(t, rows[0], &payload)
	if !payload.BySuperuser || payload.ByMigration || payload.Actor != "" || payload.Reason != "bootstrap" {
		t.Fatalf("payload = %+v", payload)
	}

	// Internal (migration / seeder) grants: no granted_by, by_migration.
	other := pbtest.NewUser(t, app, "o@test.dev")
	if err := pb.Grant(app, nil, authz.Internal("seed"), other.Id, "member", ""); err != nil {
		t.Fatalf("internal grant with nil deps: %v", err)
	}
	row = userRole(t, app, other.Id, "member")
	if row == nil || row.GetString("granted_by") != "" {
		t.Fatalf("internal grant row = %+v", row)
	}
	rows = auditRows(t, app, audit.ActionRoleGrant)
	var internalPayload audit.RoleGrantPayload
	payloadOf(t, rows[len(rows)-1], &internalPayload)
	if !internalPayload.ByMigration || internalPayload.BySuperuser || internalPayload.Actor != "internal:seed" {
		t.Fatalf("internal payload = %+v", internalPayload)
	}
}

func TestIsAdmin(t *testing.T) {
	app, d := pbtest.NewApp(t)
	if pb.IsAdmin(app, d, nil) {
		t.Fatalf("nil auth is admin")
	}
	su := pbtest.NewSuperuser(t, app, "root@test.dev")
	if !pb.IsAdmin(app, d, su) {
		t.Fatalf("superuser not admin")
	}
	u := pbtest.NewUser(t, app, "u@test.dev")
	if pb.IsAdmin(app, d, u) {
		t.Fatalf("plain user is admin")
	}
	pbtest.GrantRole(t, app, u.Id, "admin")
	if !pb.IsAdmin(app, d, u) {
		t.Fatalf("admin-role user not admin")
	}
	if !pb.IsAdmin(app, nil, u) {
		t.Fatalf("nil deps must still resolve the role")
	}
}
