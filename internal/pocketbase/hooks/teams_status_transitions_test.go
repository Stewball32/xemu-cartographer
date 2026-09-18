package hooks

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"

	"github.com/xemu-cartographer/xemu-cartographer/internal/audit"
	"github.com/xemu-cartographer/xemu-cartographer/internal/authz/pb/pbtest"
)

// teamsStatusApp is pbtest.NewApp plus the teams.status column the hook
// reads (the pbtest schema only carries name + created_by).
func teamsStatusApp(t *testing.T) core.App {
	t.Helper()
	app, _ := pbtest.NewApp(t)
	col, err := app.FindCollectionByNameOrId("teams")
	if err != nil {
		t.Fatalf("teams collection: %v", err)
	}
	if col.Fields.GetByName("status") == nil {
		col.Fields.Add(&core.SelectField{
			Name:      "status",
			Values:    []string{tmStatusApproved, tmStatusAllowed, tmStatusBlocked},
			MaxSelect: 1,
		})
		if err := app.Save(col); err != nil {
			t.Fatalf("extend teams: %v", err)
		}
	}
	return app
}

// newStatusTeam creates a team for userID with the given status.
func newStatusTeam(t *testing.T, app core.App, name, userID, status string) *core.Record {
	t.Helper()
	team := pbtest.NewTeam(t, app, name, userID)
	pbtest.SetField(t, app, "teams", team.Id, "status", status)
	return reload(t, app, team)
}

func TestTeamsStatusTransitions_InternalActorAllowed(t *testing.T) {
	app := teamsStatusApp(t)
	owner := pbtest.NewUser(t, app, "owner@test.dev")
	team := newStatusTeam(t, app, "Blue Team", owner.Id, tmStatusAllowed)
	team.Set("status", tmStatusBlocked)

	if err := teamsStatusTransitions(authzRequestEvent(app, nil, team)); err != nil {
		t.Fatalf("internal actor: %v", err)
	}
	if got := countAudit(t, app, audit.ActionBlock); got != 1 {
		t.Fatalf("ActionBlock audit rows = %d, want 1", got)
	}
}

func TestTeamsStatusTransitions_MemberDenied(t *testing.T) {
	app := teamsStatusApp(t)
	owner := memberUser(t, app, "owner@test.dev")
	team := newStatusTeam(t, app, "Blue Team", owner.Id, tmStatusAllowed)

	// The creator flipping their own team's status is what the PB rule
	// lets through and the hook must reject.
	team.Set("status", tmStatusApproved)
	wantAPIError(t, teamsStatusTransitions(authzRequestEvent(app, owner, team)), 400)
	if got := countAudit(t, app, audit.ActionApprove); got != 0 {
		t.Fatalf("ActionApprove audit rows = %d, want 0", got)
	}

	// A plain rename by the owner still passes and is audited as a
	// non-admin rename.
	rename := reload(t, app, team)
	rename.Set("name", "Red Team")
	if err := teamsStatusTransitions(authzRequestEvent(app, owner, rename)); err != nil {
		t.Fatalf("owner rename: %v", err)
	}
	rows, err := app.FindRecordsByFilter("audit_log", "action = {:a}", "", 0, 0,
		map[string]any{"a": string(audit.ActionRename)})
	if err != nil || len(rows) != 1 {
		t.Fatalf("ActionRename audit rows = %d (err %v), want 1", len(rows), err)
	}
	var payload audit.RenamePayload
	if err := rows[0].UnmarshalJSONField("payload_json", &payload); err != nil {
		t.Fatalf("rename payload: %v", err)
	}
	if payload.ByAdmin {
		t.Fatal("owner rename recorded ByAdmin = true")
	}
}

func TestTeamsStatusTransitions_AdminAllowed(t *testing.T) {
	app := teamsStatusApp(t)
	admin := adminUser(t, app, "admin@test.dev")
	owner := pbtest.NewUser(t, app, "owner@test.dev")
	team := newStatusTeam(t, app, "Blue Team", owner.Id, tmStatusAllowed)
	team.Set("status", tmStatusApproved)

	if err := teamsStatusTransitions(authzRequestEvent(app, admin, team)); err != nil {
		t.Fatalf("admin: %v", err)
	}
	if got := countAudit(t, app, audit.ActionApprove); got != 1 {
		t.Fatalf("ActionApprove audit rows = %d, want 1", got)
	}
}
