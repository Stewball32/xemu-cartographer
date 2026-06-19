package handlers

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"

	"github.com/Stewball32/xemu-cartographer/internal/guards"
)

// TestAuthorizeHostRoom_AdminBypassesGraceGate confirms the M09 grace window
// change didn't disturb the admin bypass: a superuser is admitted to a
// per-instance host room without any roster / grace involvement (svc.Scraper is
// nil and the user is in no roster).
func TestAuthorizeHostRoom_AdminBypassesGraceGate(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("NewTestApp: %v", err)
	}
	t.Cleanup(app.Cleanup)

	col, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		t.Fatalf("superusers collection: %v", err)
	}
	// A record in the _superusers collection reports IsSuperuser()==true, so
	// roles.IsAdminAuth short-circuits before the roster/grace gate. No save
	// needed — the flag is collection-derived.
	su := core.NewRecord(col)

	e := &Event{
		Room:     "host:pod-a",
		Services: &guards.Services{App: app},
		User:     su,
	}
	if err := authorizeHostRoom(e); err != nil {
		t.Errorf("admin/superuser should bypass the roster+grace gate, got: %v", err)
	}
}
