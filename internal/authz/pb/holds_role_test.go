package pb_test

import (
	"errors"
	"testing"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/xemu-cartographer/xemu-cartographer/internal/authz"
	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/pb"
	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/pb/pbtest"
)

// flakyApp is a core.App whose user_roles reads fail while the matching
// budget is above zero — the transient lookup failure denyLastAdmin must
// refuse on rather than read as "does not hold". Transactions hand the
// callback a wrapped txApp sharing the budgets so the failure lands inside
// Revoke's transaction, where the adapter actually looks.
type flakyApp struct {
	core.App
	failFirst *int // FindFirstRecordByFilter("user_roles", …) — HoldsRole
	failList  *int // FindRecordsByFilter("user_roles", …)  — AdminCount
}

var errFlaky = errors.New("simulated user_roles lookup failure")

func newFlakyApp(app core.App, failFirst, failList int) *flakyApp {
	return &flakyApp{App: app, failFirst: &failFirst, failList: &failList}
}

func (f *flakyApp) FindFirstRecordByFilter(coll any, filter string, params ...dbx.Params) (*core.Record, error) {
	if coll == "user_roles" && *f.failFirst > 0 {
		*f.failFirst--
		return nil, errFlaky
	}
	return f.App.FindFirstRecordByFilter(coll, filter, params...)
}

func (f *flakyApp) FindRecordsByFilter(coll any, filter, sort string, limit, offset int, params ...dbx.Params) ([]*core.Record, error) {
	if coll == "user_roles" && *f.failList > 0 {
		*f.failList--
		return nil, errFlaky
	}
	return f.App.FindRecordsByFilter(coll, filter, sort, limit, offset, params...)
}

func (f *flakyApp) RunInTransaction(fn func(core.App) error) error {
	return f.App.RunInTransaction(func(tx core.App) error {
		return fn(&flakyApp{App: tx, failFirst: f.failFirst, failList: f.failList})
	})
}

// TestHoldsRoleAnswers pins the (holds, ok) contract of the adapter:
// definitive answers for a holder, a non-holder and an unknown slug;
// ok=false for a nil adapter, empty arguments and a failed lookup.
func TestHoldsRoleAnswers(t *testing.T) {
	app, d := pbtest.NewApp(t)
	holder := pbtest.NewUser(t, app, "holder@test.dev")
	pbtest.GrantRole(t, app, holder.Id, "admin")
	other := pbtest.NewUser(t, app, "other@test.dev")

	check := func(name string, d *pb.PBDeps, userID, slug string, wantHolds, wantOK bool) {
		t.Helper()
		if holds, ok := d.HoldsRole(userID, slug); holds != wantHolds || ok != wantOK {
			t.Errorf("%s: HoldsRole(%q, %q) = %v,%v, want %v,%v", name, userID, slug, holds, ok, wantHolds, wantOK)
		}
	}
	check("holder", d, holder.Id, "admin", true, true)
	check("holder, other role", d, holder.Id, "member", false, true)
	check("non-holder", d, other.Id, "admin", false, true)
	check("unknown slug", d, holder.Id, "nosuchrole", false, true)
	check("unknown user", d, "nosuchuser000000", "admin", false, true)
	check("empty user", d, "", "admin", false, false)
	check("empty slug", d, holder.Id, "", false, false)
	check("nil adapter", (*pb.PBDeps)(nil), holder.Id, "admin", false, false)

	// A failed user_roles read is unknown, not "does not hold"; once the
	// read works again the answer is definitive.
	flaky := newFlakyApp(app, 1, 0)
	fd := pb.WithApp(d, flaky)
	check("lookup failed", fd, holder.Id, "admin", false, false)
	check("lookup recovered", fd, holder.Id, "admin", true, true)
}

// TestRevokeLastAdminLookupFailure pins the fail-closed side of the
// last-admin guard at the adapter: when the holder lookup (and, in the
// second case, the admin count too) fails inside Revoke's transaction, the
// revoke is refused with ErrLastAdmin and the last admin's row survives —
// a broken lookup must never be read as "the target is not an admin".
func TestRevokeLastAdminLookupFailure(t *testing.T) {
	for _, tc := range []struct {
		name                string
		failFirst, failList int
	}{
		{"holder lookup fails", 1, 0},
		{"holder lookup and admin count fail", 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, d := pbtest.NewApp(t)
			only := pbtest.NewUser(t, app, "only@test.dev")
			pbtest.GrantRole(t, app, only.Id, "admin")
			if n := d.AdminCount(); n != 1 {
				t.Fatalf("AdminCount = %d", n)
			}

			flaky := newFlakyApp(app, tc.failFirst, tc.failList)
			err := pb.Revoke(flaky, d, authz.Superuser("root"), only.Id, "admin", "oops")
			if !errors.Is(err, authz.ErrLastAdmin) {
				t.Fatalf("revoke with a failing holder lookup: err = %v, want ErrLastAdmin", err)
			}
			if *flaky.failFirst != 0 {
				t.Fatalf("the holder lookup never ran (budget left %d)", *flaky.failFirst)
			}
			if userRole(t, app, only.Id, "admin") == nil {
				t.Fatalf("last admin row deleted despite the failed lookup")
			}
			if n := d.AdminCount(); n != 1 {
				t.Fatalf("AdminCount after refused revoke = %d", n)
			}

			// The same revoke against a healthy app is still refused, and a
			// non-holder's revoke is still the usual no-op — the guard did
			// not tip over into refusing everything.
			if err := pb.Revoke(app, d, authz.Superuser("root"), only.Id, "admin", "oops"); !errors.Is(err, authz.ErrLastAdmin) {
				t.Fatalf("healthy revoke of the last admin: %v", err)
			}
			bystander := pbtest.NewUser(t, app, "bystander@test.dev")
			if err := pb.Revoke(newFlakyApp(app, 0, 0), d, authz.Superuser("root"), bystander.Id, "admin", ""); err != nil {
				t.Fatalf("no-op revoke of a non-holder through the wrapper: %v", err)
			}
		})
	}
}
