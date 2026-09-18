package adminusers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/pb"
	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/pb/pbtest"
)

// TestGrantAnonymousIs400 pins the roleMutationError mapping for
// pb.ErrRoleNotGrantable: granting the "anonymous" slug (the console door's
// scope list, PD-1) is refused by pb.Grant for every actor, and the route
// answers 400 like the slug pre-check — not the historical 500 — with no
// user_roles row written.
func TestGrantAnonymousIs400(t *testing.T) {
	app, _ := pbtest.NewApp(t)

	admin := pbtest.NewUser(t, app, "admin@test.dev")
	pbtest.GrantRole(t, app, admin.Id, "admin")
	target := pbtest.NewUser(t, app, "target@test.dev")

	req := httptest.NewRequest(http.MethodPost, "/api/admin/users/"+target.Id+"/roles", nil)
	e := &core.RequestEvent{
		App:   app,
		Auth:  admin,
		Event: router.Event{Request: req, Response: httptest.NewRecorder()},
	}

	err := pb.Grant(app, pb.Default(), pb.Get(e), target.Id, "anonymous", "")
	if !errors.Is(err, pb.ErrRoleNotGrantable) {
		t.Fatalf("pb.Grant(anonymous) = %v, want ErrRoleNotGrantable", err)
	}

	var apiErr *router.ApiError
	if mapped := roleMutationError(e, "grant", target.Id, "anonymous", err); !errors.As(mapped, &apiErr) {
		t.Fatalf("roleMutationError returned a non-API error: %v", mapped)
	}
	if apiErr.Status != http.StatusBadRequest {
		t.Errorf("status = %d (%q), want 400", apiErr.Status, apiErr.Message)
	}

	rows, err := app.FindAllRecords("user_roles", nil)
	if err != nil {
		t.Fatalf("list user_roles: %v", err)
	}
	for _, r := range rows {
		if r.GetString("user") == target.Id {
			t.Fatalf("refused grant wrote a user_roles row: %v", r)
		}
	}
}
