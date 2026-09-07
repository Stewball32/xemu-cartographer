package pb_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"

	"github.com/Stewball32/xemu-cartographer/internal/authz/pb"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb/pbtest"
)

// requireAuthMux builds a router the way main.go wires it: RejectBannedAuth
// router-wide, then a group guarded only by apis.RequireAuth() whose handler
// echoes the auth id. auth plays the part of PocketBase's JWT loader.
func requireAuthMux(t *testing.T, app core.App, auth *core.Record) http.Handler {
	t.Helper()
	r := router.NewRouter(func(w http.ResponseWriter, req *http.Request) (*core.RequestEvent, router.EventCleanupFunc) {
		return &core.RequestEvent{App: app, Auth: auth, Event: router.Event{Request: req, Response: w}}, nil
	})
	r.BindFunc(pb.RejectBannedAuth)
	g := r.Group("/api")
	g.Bind(apis.RequireAuth())
	g.GET("/me", func(e *core.RequestEvent) error {
		return e.String(http.StatusOK, e.Auth.Id)
	})
	mux, err := r.BuildMux()
	if err != nil {
		t.Fatalf("BuildMux: %v", err)
	}
	return mux
}

func TestRejectBannedAuth(t *testing.T) {
	app, _ := pbtest.NewApp(t)
	user := pbtest.NewUser(t, app, "rb@test.dev")
	su := pbtest.NewSuperuser(t, app, "root@test.dev")

	get := func(auth *core.Record) (int, string) {
		rec := httptest.NewRecorder()
		requireAuthMux(t, app, auth).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/me", nil))
		return rec.Code, rec.Body.String()
	}

	if code, body := get(user); code != http.StatusOK || body != user.Id {
		t.Fatalf("active user: %d %s", code, body)
	}
	if code, _ := get(nil); code != http.StatusUnauthorized {
		t.Fatalf("guest: %d", code)
	}

	// The middleware re-reads the row: the JWT record in hand still says
	// is_banned=false.
	pbtest.SetField(t, app, "users", user.Id, "is_banned", true)
	if code, _ := get(user); code != http.StatusUnauthorized {
		t.Fatalf("banned user on a RequireAuth-only route: %d", code)
	}
	pbtest.SetField(t, app, "users", user.Id, "banned_until", time.Now().Add(-time.Hour))
	if code, body := get(user); code != http.StatusOK || body != user.Id {
		t.Fatalf("lapsed ban: %d %s", code, body)
	}
	pbtest.SetField(t, app, "users", user.Id, "deleted_at", time.Now())
	if code, _ := get(user); code != http.StatusUnauthorized {
		t.Fatalf("soft-deleted user: %d", code)
	}
	// Superusers are never touched.
	if code, body := get(su); code != http.StatusOK || body != su.Id {
		t.Fatalf("superuser: %d %s", code, body)
	}
	// A row that no longer exists is unusable too.
	e := newEvent(app, user, http.MethodGet, "/api/me", nil)
	if err := app.Delete(user); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	if err := pb.RejectBannedAuth(e); err != nil || e.Auth != nil {
		t.Fatalf("deleted row: auth=%v err=%v", e.Auth, err)
	}
}
