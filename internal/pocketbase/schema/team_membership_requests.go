package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// registerTeamMembershipRequestsCollection creates the
// team_membership_requests collection landed in M23c — the directed pending
// state for invites (owner/manager → user) and join requests (user → team).
// Resolved by routes under /api/team-membership-requests/{id}/accept (M23d),
// .../decline, .../cancel.
//
// No `gamertag` column. Per the M23 decision tree: default_gamertag is the
// social handle, so the accept route resolves it server-side at acceptance
// time. Storing a gamertag pick on the request would force a UI picker the
// recipient doesn't need.
//
// expires_at stays nullable; the `expired` status string is reserved but
// unreachable until a future cron promotes pending rows past their TTL.
// The M23 doc explicitly defers that cron.
//
// PB rules: list/view = recipient OR initiator OR admin. Mutations all nil
// — creates flow through the invite/join_request routes; status transitions
// flow through accept/decline/cancel routes (M23d). Direct PB SDK writes
// would bypass guard checks (existing roster, pending duplicate, blocked
// team).
func registerTeamMembershipRequestsCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "team_membership_requests") {
		return reconcileTeamMembershipRequestsRules(app)
	}

	collection := core.NewBaseCollection("team_membership_requests")

	teamsCol, err := requireCollection(app, "teams")
	if err != nil {
		return err
	}
	usersCol, err := requireCollection(app, "users")
	if err != nil {
		return err
	}

	collection.Fields.Add(
		&core.RelationField{
			Name:          "team",
			Required:      true,
			CollectionId:  teamsCol.Id,
			MaxSelect:     1,
			CascadeDelete: false,
		},
		&core.RelationField{
			Name:          "user",
			Required:      true,
			CollectionId:  usersCol.Id,
			MaxSelect:     1,
			CascadeDelete: false,
		},
		&core.SelectField{
			Name:     "direction",
			Required: true,
			Values:   teamMembershipRequestDirectionValues,
		},
		&core.SelectField{
			Name:     "status",
			Required: true,
			Values:   teamMembershipRequestStatusValues,
		},
		&core.RelationField{
			Name:          "initiated_by",
			Required:      true,
			CollectionId:  usersCol.Id,
			MaxSelect:     1,
			CascadeDelete: false,
		},
		&core.RelationField{
			Name:          "responded_by",
			CollectionId:  usersCol.Id,
			MaxSelect:     1,
			CascadeDelete: false,
		},
		&core.DateField{Name: "expires_at"},
		&core.AutodateField{
			Name:     "created",
			OnCreate: true,
		},
		&core.AutodateField{
			Name:     "updated",
			OnCreate: true,
			OnUpdate: true,
		},
	)

	// Pending-duplicate guard: filter (team, user, status="pending")
	// — both invite + join_request route handlers walk this index before
	// inserting.
	// "My inbox" walk for /settings/ pending sections: filter
	// (user, status="pending"), sort -created.
	collection.AddIndex("idx_tmr_team_user_status", false, "team, user, status", "")
	collection.AddIndex("idx_tmr_user_status_created", false, "user, status, created DESC", "")

	setTeamMembershipRequestsRules(collection)

	return app.Save(collection)
}

// teamMembershipRequestDirectionValues — owner/manager → user is "invited";
// user → team is "requested". Stored as a SelectField allow-list rather
// than a free text column so a typo at write time fails loudly.
var teamMembershipRequestDirectionValues = []string{"invited", "requested"}

// teamMembershipRequestStatusValues — the lifecycle states. `expired` is
// reserved for the deferred cron (see doc comment above); the M23 routes
// only write `pending` (on create) and the four resolution states.
var teamMembershipRequestStatusValues = []string{"pending", "accepted", "declined", "expired", "cancelled"}

func reconcileTeamMembershipRequestsRules(app *pocketbase.PocketBase) error {
	existing, err := app.FindCollectionByNameOrId("team_membership_requests")
	if err != nil {
		return err
	}
	setTeamMembershipRequestsRules(existing)
	return app.Save(existing)
}

func setTeamMembershipRequestsRules(c *core.Collection) {
	read := "user = @request.auth.id || initiated_by = @request.auth.id || (" + hasAdminRole + ")"

	c.ListRule = &read
	c.ViewRule = &read
	c.CreateRule = nil
	c.UpdateRule = nil
	c.DeleteRule = nil
}
