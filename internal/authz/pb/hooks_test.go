package pb_test

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"

	"github.com/xemu-cartographer/xemu-cartographer/internal/audit"
	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/pb"
	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/pb/pbtest"
)

// roleRow finds the roles row by slug.
func roleRow(t *testing.T, app core.App, slug string) *core.Record {
	t.Helper()
	rec, err := app.FindFirstRecordByData("roles", "slug", slug)
	if err != nil {
		t.Fatalf("roles/%s: %v", slug, err)
	}
	return rec
}

func TestRolesHooksInvalidateCache(t *testing.T) {
	app, d := pbtest.NewApp(t)
	// Warm the cache, then edit the rows directly (no InvalidateRoles call —
	// that is the hooks' job).
	if lvl, ok := d.RoleLevel("organizer"); !ok || lvl != 50 {
		t.Fatalf("organizer level = %d %v", lvl, ok)
	}
	if got := d.AnonymousScopes(); len(got) == 0 {
		t.Fatalf("anonymous scopes empty before the edit")
	}

	org := roleRow(t, app, "organizer")
	org.Set("level", 55)
	if err := app.Save(org); err != nil {
		t.Fatalf("save organizer: %v", err)
	}
	if lvl, ok := d.RoleLevel("organizer"); !ok || lvl != 55 {
		t.Fatalf("organizer level after edit = %d %v, want 55", lvl, ok)
	}

	anon := roleRow(t, app, "anonymous")
	anon.Set("scopes", []string{"overlay.read_state:xc-1"})
	if err := app.Save(anon); err != nil {
		t.Fatalf("save anonymous: %v", err)
	}
	if got := d.AnonymousScopes(); len(got) != 1 || got[0] != "overlay.read_state:xc-1" {
		t.Fatalf("anonymous scopes after edit = %v", got)
	}

	// A new row is visible at once …
	col := roleRow(t, app, "member").Collection()
	streamer := core.NewRecord(col)
	streamer.Set("slug", "streamer")
	streamer.Set("label", "Streamer")
	streamer.Set("level", 30)
	streamer.Set("scopes", []string{"overlay.read_state:*"})
	if err := app.Save(streamer); err != nil {
		t.Fatalf("save streamer: %v", err)
	}
	if lvl, ok := d.RoleLevel("streamer"); !ok || lvl != 30 {
		t.Fatalf("new role level = %d %v, want 30", lvl, ok)
	}
	// … and so is its removal.
	if err := app.Delete(streamer); err != nil {
		t.Fatalf("delete streamer: %v", err)
	}
	if _, ok := d.RoleLevel("streamer"); ok {
		t.Fatalf("deleted role still cached")
	}
}

func TestBoundTokensRevokedOnBindingDelete(t *testing.T) {
	app, d := pbtest.NewApp(t)
	owner := pbtest.NewUser(t, app, "owner@test.dev")
	tag := pbtest.NewTag(t, app, owner.Id, "Station7", "approved")
	keep := pbtest.NewTag(t, app, owner.Id, "Station8", "approved")

	bound, _ := pbtest.MintToken(t, app, d, "machine", []string{"lan.saves.*"}, func(r *pb.MintRequest) {
		r.Gamertags = []string{tag.Id}
	})
	both, _ := pbtest.MintToken(t, app, d, "machine", []string{"lan.saves.*"}, func(r *pb.MintRequest) {
		r.Gamertags = []string{tag.Id, keep.Id}
	})
	other, _ := pbtest.MintToken(t, app, d, "machine", []string{"lan.saves.*"}, func(r *pb.MintRequest) {
		r.Gamertags = []string{keep.Id}
	})
	free, _ := pbtest.MintToken(t, app, d, "machine", []string{"lan.saves.*"})
	if row, _ := d.LookupToken(bound); len(row.Gamertags) != 1 || row.Gamertags[0] != "station7" {
		t.Fatalf("bound row = %+v", row)
	}

	if err := app.Delete(tag); err != nil {
		t.Fatalf("delete gamertag: %v", err)
	}
	for _, kid := range []string{bound, both} {
		row, ok := d.LookupToken(kid)
		if !ok || !row.Revoked {
			t.Fatalf("%s after its gamertag was deleted = %+v %v, want Revoked", kid, row, ok)
		}
		rec := pbtest.TokenRecord(t, app, kid)
		if rec.GetDateTime("revoked_at").IsZero() || rec.GetString("revoked_by") != "" {
			t.Fatalf("%s revoked_at/revoked_by = %s/%q", kid, rec.GetDateTime("revoked_at"), rec.GetString("revoked_by"))
		}
	}
	for _, kid := range []string{other, free} {
		if row, ok := d.LookupToken(kid); !ok || row.Revoked {
			t.Fatalf("%s (not bound to the deleted gamertag) = %+v %v", kid, row, ok)
		}
	}
	rows := auditRows(t, app, audit.ActionTokenRevoke)
	if len(rows) != 2 {
		t.Fatalf("token_revoke audit rows = %d, want 2", len(rows))
	}
	seen := map[string]bool{}
	for _, r := range rows {
		var payload audit.TokenRevokePayload
		payloadOf(t, r, &payload)
		if payload.Reason != "binding deleted: gamertags/"+tag.Id || payload.Actor != "internal:hook" || payload.BySuperuser {
			t.Fatalf("payload = %+v", payload)
		}
		if r.GetString("actor") != "" {
			t.Fatalf("hook revoke has an actor: %q", r.GetString("actor"))
		}
		seen[payload.Kid] = true
	}
	if !seen[bound] || !seen[both] {
		t.Fatalf("audited kids = %v", seen)
	}

	// A key linked to a user goes with the user.
	linked := pbtest.NewUser(t, app, "linked@test.dev")
	mine, _ := pbtest.MintToken(t, app, d, "machine", []string{"lan.saves.*"}, func(r *pb.MintRequest) {
		r.UserID = linked.Id
	})
	if err := app.Delete(linked); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	row, ok := d.LookupToken(mine)
	if !ok || !row.Revoked {
		t.Fatalf("%s after its user was deleted = %+v %v, want Revoked", mine, row, ok)
	}
	if row, ok := d.LookupToken(other); !ok || row.Revoked {
		t.Fatalf("%s revoked by an unrelated user delete: %+v %v", other, row, ok)
	}
	rows = auditRows(t, app, audit.ActionTokenRevoke)
	if len(rows) != 3 {
		t.Fatalf("token_revoke audit rows = %d, want 3", len(rows))
	}
	var payload audit.TokenRevokePayload
	payloadOf(t, rows[2], &payload)
	if payload.Kid != mine || payload.Reason != "binding deleted: users/"+linked.Id {
		t.Fatalf("user-delete payload = %+v", payload)
	}
}

func TestBoundTokensRevokedOnUserSoftDelete(t *testing.T) {
	app, d := pbtest.NewApp(t)
	user := pbtest.NewUser(t, app, "leaving@test.dev")
	bystander := pbtest.NewUser(t, app, "staying@test.dev")
	mine, _ := pbtest.MintToken(t, app, d, "machine", []string{"lan.saves.*"}, func(r *pb.MintRequest) {
		r.UserID = user.Id
	})
	theirs, _ := pbtest.MintToken(t, app, d, "machine", []string{"lan.saves.*"}, func(r *pb.MintRequest) {
		r.UserID = bystander.Id
	})

	// A ban is reversible and must not burn the key.
	user.Set("is_banned", true)
	if err := app.Save(user); err != nil {
		t.Fatalf("ban: %v", err)
	}
	if row, ok := d.LookupToken(mine); !ok || row.Revoked {
		t.Fatalf("%s after a ban = %+v %v, want live", mine, row, ok)
	}
	if got := auditRows(t, app, audit.ActionTokenRevoke); len(got) != 0 {
		t.Fatalf("ban wrote %d token_revoke audit rows", len(got))
	}

	// The soft-delete transition revokes it, inside the same save.
	user.Set("is_deleted", true)
	if err := app.Save(user); err != nil {
		t.Fatalf("soft-delete: %v", err)
	}
	row, ok := d.LookupToken(mine)
	if !ok || !row.Revoked {
		t.Fatalf("%s after its user was soft-deleted = %+v %v, want Revoked", mine, row, ok)
	}
	rec := pbtest.TokenRecord(t, app, mine)
	if rec.GetDateTime("revoked_at").IsZero() || rec.GetString("revoked_by") != "" {
		t.Fatalf("revoked_at/revoked_by = %s/%q", rec.GetDateTime("revoked_at"), rec.GetString("revoked_by"))
	}
	if row, ok := d.LookupToken(theirs); !ok || row.Revoked {
		t.Fatalf("%s (another user's key) = %+v %v", theirs, row, ok)
	}
	rows := auditRows(t, app, audit.ActionTokenRevoke)
	if len(rows) != 1 {
		t.Fatalf("token_revoke audit rows = %d, want 1", len(rows))
	}
	var payload audit.TokenRevokePayload
	payloadOf(t, rows[0], &payload)
	if payload.Kid != mine || payload.Reason != "binding soft-deleted: users/"+user.Id || payload.Actor != "internal:hook" {
		t.Fatalf("soft-delete payload = %+v", payload)
	}

	// Re-saving an already soft-deleted row is not a second transition.
	user.Set("is_banned", false)
	if err := app.Save(user); err != nil {
		t.Fatalf("re-save: %v", err)
	}
	if got := auditRows(t, app, audit.ActionTokenRevoke); len(got) != 1 {
		t.Fatalf("re-save wrote more token_revoke audit rows: %d", len(got))
	}

	// deleted_at alone is the other marker the resolver honours.
	other := pbtest.NewUser(t, app, "stamped@test.dev")
	stamped, _ := pbtest.MintToken(t, app, d, "machine", []string{"lan.saves.*"}, func(r *pb.MintRequest) {
		r.UserID = other.Id
	})
	other.Set("deleted_at", "2026-09-05 00:00:00.000Z")
	if err := app.Save(other); err != nil {
		t.Fatalf("stamp deleted_at: %v", err)
	}
	if row, ok := d.LookupToken(stamped); !ok || !row.Revoked {
		t.Fatalf("%s after deleted_at was stamped = %+v %v, want Revoked", stamped, row, ok)
	}
}
