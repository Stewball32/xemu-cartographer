package pb_test

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"

	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/pb"
	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/pb/pbtest"
)

func TestTouchLastUsedNeverClobbersRevoke(t *testing.T) {
	app, d := pbtest.NewApp(t)
	live, _ := pbtest.MintToken(t, app, d, "machine", []string{"lan.saves.*"})
	gone, _ := pbtest.MintToken(t, app, d, "machine", []string{"lan.saves.*"})

	// The stamp is a column UPDATE, never a record save: a save of a row
	// loaded before a racing revoke would write the stale revoked=false back.
	saves := 0
	app.OnRecordUpdate("api_tokens").BindFunc(func(e *core.RecordEvent) error {
		saves++
		return e.Next()
	})

	before := pbtest.TokenRecord(t, app, live)
	pb.TouchLastUsed(d, app, live)
	after := pbtest.TokenRecord(t, app, live)
	if after.GetDateTime("last_used_at").IsZero() {
		t.Fatalf("live key not stamped")
	}
	if saves != 0 {
		t.Fatalf("touch saved the record (%d api_tokens update hooks fired)", saves)
	}
	for _, col := range []string{"key_hash", "scopes", "revoked", "revoked_at", "expires_at", "label"} {
		if before.GetString(col) != after.GetString(col) {
			t.Fatalf("touch changed %s: %q → %q", col, before.GetString(col), after.GetString(col))
		}
	}

	// A key revoked after the resolver last saw it is never stamped and
	// stays revoked; LookupToken keeps reporting it.
	rec := pbtest.TokenRecord(t, app, gone)
	pbtest.SetField(t, app, "api_tokens", rec.Id, "revoked", true)
	saves = 0 // SetField is itself a record save
	pb.TouchLastUsed(d, app, gone)
	rec = pbtest.TokenRecord(t, app, gone)
	if !rec.GetBool("revoked") {
		t.Fatalf("touch un-revoked the key")
	}
	if !rec.GetDateTime("last_used_at").IsZero() {
		t.Fatalf("revoked key was stamped: %s", rec.GetDateTime("last_used_at"))
	}
	if row, ok := d.LookupToken(gone); !ok || !row.Revoked {
		t.Fatalf("LookupToken after touch = %+v %v, want Revoked", row, ok)
	}
	if saves != 0 {
		t.Fatalf("touch of a revoked key saved the record (%d hooks)", saves)
	}
}

func TestRosteredIn(t *testing.T) {
	fake := &pbtest.FakeScraper{}
	app, d := pbtest.NewAppWith(t, fake, nil)
	user := pbtest.NewUser(t, app, "ros@test.dev")
	other := pbtest.NewUser(t, app, "other@test.dev")
	// Container names are unique to this test: the grace tracker is the
	// process-wide rostergrace.Default.
	const box, spare = "xc-rosteredin-1", "xc-rosteredin-2"

	if d.RosteredIn(user.Id, box) {
		t.Fatalf("user without gamertags rostered")
	}
	pbtest.NewTag(t, app, user.Id, "Approved Tag", "approved")
	pbtest.NewTag(t, app, user.Id, "PendingTag", "pending")
	pbtest.NewTag(t, app, user.Id, "BlockedTag", "blocked")
	pbtest.NewTag(t, app, other.Id, "OtherTag", "allowed")

	if d.RosteredIn(user.Id, box) {
		t.Fatalf("rostered with no live membership")
	}
	// Pending / blocked tags in the roster never unlock the box.
	fake.SetMembership(box, "pendingtag", "blockedtag")
	if d.RosteredIn(user.Id, box) {
		t.Fatalf("pending / blocked tag unlocked the box")
	}
	// An approved tag does (identities are matched sanitized).
	fake.SetMembership(box, "pendingtag", "APPROVED TAG")
	if !d.RosteredIn(user.Id, box) {
		t.Fatalf("approved rostered tag refused")
	}
	// Only for the box it is in.
	fake.SetMembership(spare, "othertag")
	if d.RosteredIn(user.Id, spare) {
		t.Fatalf("rostered in a box holding someone else's tag")
	}
	if !d.RosteredIn(other.Id, spare) || d.RosteredIn(other.Id, box) {
		t.Fatalf("other user's boxes wrong")
	}
	// The grace window keeps the box open right after the tag drops out …
	fake.SetMembership(box)
	if !d.RosteredIn(user.Id, box) {
		t.Fatalf("grace window not honoured after a live sighting")
	}
	// … but a box the tag was never seen in stays closed.
	if d.RosteredIn(user.Id, "xc-rosteredin-3") {
		t.Fatalf("grace granted for a never-seen box")
	}
	// Inputs the adapter cannot answer deny.
	if d.RosteredIn("", box) || d.RosteredIn(user.Id, "") {
		t.Fatalf("empty user / instance rostered")
	}
	var nilDeps *pb.PBDeps
	if nilDeps.RosteredIn(user.Id, box) {
		t.Fatalf("nil deps rostered")
	}
	_, noScraper := pbtest.NewAppWith(t, nil, nil)
	if noScraper.RosteredIn(user.Id, box) {
		t.Fatalf("nil scraper rostered")
	}
}
