package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// registerTeamLogCollection creates the team_log collection landed in M23c —
// the team-centric activity timeline. Captures membership churn
// (invite/request/accept/decline/cancel/remove/leave) and team-shape events
// (rename, status change) so the public team page can render an "Activity"
// section without joining audit_log (which stays admin-only per M22).
//
// Why a separate collection from audit_log? Per the M23 plan, audit_log is
// the admin/moderation surface (cross-collection, admin-only read). team_log
// is the team-internal activity surface — readable by any authed user. Two
// collections, two read-permission shapes, no muxing inside one table.
//
// The renames written here are duplicated with the audit_log ActionRename
// row M22d already emits. Twin-writes are intentional: audit_log keeps its
// admin-only consumer, team_log feeds the public team page. Storage cost is
// trivial; the alternative (single shared collection with mixed read
// permissions) would block M23's public timeline behind admin.
//
// payload_json carries the per-event shape declared in internal/teamlog/.
// The Write helper there owns the marshaling so direct PB SDK writes don't
// silently corrupt rows.
//
// PB rules: list/view = any authed user (events are team-scoped public
// metadata in this community's scope — same access level as teams + rosters
// + reserved_names). Create/update/delete = nil — every legitimate write
// flows through teamlog.Write which bypasses rules via app.Save.
func registerTeamLogCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "team_log") {
		return reconcileTeamLogRules(app)
	}

	collection := core.NewBaseCollection("team_log")

	teamsCol, err := requireCollection(app, "teams")
	if err != nil {
		return err
	}
	usersCol, err := requireCollection(app, "users")
	if err != nil {
		return err
	}
	gamertagsCol, err := requireCollection(app, "gamertags")
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
		&core.TextField{
			Name:     "event",
			Required: true,
			Min:      1,
		},
		&core.RelationField{
			Name:          "actor",
			CollectionId:  usersCol.Id,
			MaxSelect:     1,
			CascadeDelete: false,
		},
		&core.RelationField{
			Name:          "subject_user",
			CollectionId:  usersCol.Id,
			MaxSelect:     1,
			CascadeDelete: false,
		},
		&core.RelationField{
			Name:          "subject_gamertag",
			CollectionId:  gamertagsCol.Id,
			MaxSelect:     1,
			CascadeDelete: false,
		},
		&core.JSONField{
			Name:    "payload_json",
			MaxSize: 1 << 18, // 256 KiB
		},
		&core.AutodateField{
			Name:     "created",
			OnCreate: true,
		},
	)

	// Three read patterns the UI hits:
	//   (1) full team timeline   — filter (team), sort -created
	//   (2) team timeline by event class (e.g. just renames) — filter
	//       (team, event), sort -created
	//   (3) cross-team subject history (a user's membership churn across
	//       every team) — filter (subject_user), sort -created
	collection.AddIndex("idx_team_log_team_created", false, "team, created DESC", "")
	collection.AddIndex("idx_team_log_team_event_created", false, "team, event, created DESC", "")
	collection.AddIndex("idx_team_log_subject_user_created", false, "subject_user, created DESC", "")

	setTeamLogRules(collection)

	return app.Save(collection)
}

func reconcileTeamLogRules(app *pocketbase.PocketBase) error {
	existing, err := app.FindCollectionByNameOrId("team_log")
	if err != nil {
		return err
	}
	setTeamLogRules(existing)
	return app.Save(existing)
}

func setTeamLogRules(c *core.Collection) {
	listView := "@request.auth.id != \"\""

	c.ListRule = &listView
	c.ViewRule = &listView
	c.CreateRule = nil
	c.UpdateRule = nil
	c.DeleteRule = nil
}
