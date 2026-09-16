package hooks

import (
	"testing"

	"github.com/Stewball32/xemu-cartographer/internal/audit"
	"github.com/Stewball32/xemu-cartographer/internal/authz/pb/pbtest"
)

func TestGamertagsStatusTransitions_InternalActorAllowed(t *testing.T) {
	app, _ := pbtest.NewApp(t)
	owner := pbtest.NewUser(t, app, "owner@test.dev")
	tag := reload(t, app, pbtest.NewTag(t, app, owner.Id, "StewGoal", gtStatusAllowed))
	tag.Set("status", gtStatusBlocked)

	if err := gamertagsStatusTransitions(authzRequestEvent(app, nil, tag)); err != nil {
		t.Fatalf("internal actor: %v", err)
	}
	if got := countAudit(t, app, audit.ActionBlock); got != 1 {
		t.Fatalf("ActionBlock audit rows = %d, want 1", got)
	}
}

func TestGamertagsStatusTransitions_MemberDenied(t *testing.T) {
	app, _ := pbtest.NewApp(t)
	owner := memberUser(t, app, "owner@test.dev")
	tag := reload(t, app, pbtest.NewTag(t, app, owner.Id, "StewGoal", gtStatusAllowed))

	// The owner flipping their own row's status is the exact case the PB
	// rule lets through and the hook must reject.
	tag.Set("status", gtStatusApproved)
	wantAPIError(t, gamertagsStatusTransitions(authzRequestEvent(app, owner, tag)), 400)
	if got := countAudit(t, app, audit.ActionApprove); got != 0 {
		t.Fatalf("ActionApprove audit rows = %d, want 0", got)
	}

	// A member editing only the tag text is not a moderation write.
	edit := reload(t, app, tag)
	edit.Set("tag", "StewGoal2")
	if err := gamertagsStatusTransitions(authzRequestEvent(app, owner, edit)); err != nil {
		t.Fatalf("tag-only edit: %v", err)
	}
}

func TestGamertagsStatusTransitions_AdminAllowed(t *testing.T) {
	app, _ := pbtest.NewApp(t)
	admin := adminUser(t, app, "admin@test.dev")
	owner := pbtest.NewUser(t, app, "owner@test.dev")
	tag := reload(t, app, pbtest.NewTag(t, app, owner.Id, "StewGoal", gtStatusAllowed))
	tag.Set("status", gtStatusApproved)

	if err := gamertagsStatusTransitions(authzRequestEvent(app, admin, tag)); err != nil {
		t.Fatalf("admin: %v", err)
	}
	if got := countAudit(t, app, audit.ActionApprove); got != 1 {
		t.Fatalf("ActionApprove audit rows = %d, want 1", got)
	}
}
