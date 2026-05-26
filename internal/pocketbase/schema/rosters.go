package schema

import (
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// registerRostersCollection creates the rosters collection — each row is one
// stint of a (team, gamertag) pair. left_at = null means the stint is active;
// flipping left_at to a date "ends" the stint without losing history. The
// same (team, gamertag) may appear in multiple rows over time (re-joins),
// modeled after Liquipedia's "Former" tables.
//
// Cascade is intentionally disabled on both relations: when a gamertag is
// deleted (or a team folded) we keep the roster rows so the history table is
// stable. Code that displays rosters tolerates missing tags/teams; admins
// who really need to purge can blank the relation columns directly.
func registerRostersCollection(app *pocketbase.PocketBase) error {
	if collectionExists(app, "rosters") {
		return reconcileRostersRules(app)
	}

	collection := core.NewBaseCollection("rosters")

	teamsCol, err := requireCollection(app, "teams")
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
		&core.RelationField{
			Name:          "gamertag",
			Required:      true,
			CollectionId:  gamertagsCol.Id,
			MaxSelect:     1,
			CascadeDelete: false,
		},
		&core.BoolField{Name: "is_captain"},
		&core.BoolField{Name: "is_manager"},
		&core.DateField{
			Name:     "joined_at",
			Required: true,
		},
		&core.DateField{
			Name: "left_at",
		},
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

	// Active-roster lookups (team -> currently-rostered players) and
	// player-history lookups (gamertag -> all stints) are the two read
	// patterns the UI hits most. No uniqueness — re-joins are first-class.
	collection.AddIndex("idx_rosters_team_left_at", false, "team, left_at", "")
	collection.AddIndex("idx_rosters_gamertag", false, "gamertag", "")

	setRostersRules(collection)

	return app.Save(collection)
}

func reconcileRostersRules(app *pocketbase.PocketBase) error {
	existing, err := app.FindCollectionByNameOrId("rosters")
	if err != nil {
		return err
	}
	setRostersRules(existing)
	return app.Save(existing)
}

// setRostersRules:
//   - List/View: any authed user — rosters are public-ish team metadata.
//   - Create/Update/Delete: team creator (via team.created_by relation
//     chain) OR admin. Captain/manager mutation rights are deferred to a
//     custom route layer — see the comment in setTeamsRules.
//
// Relation chaining (`team.created_by`) is supported by PB rules and side-
// steps the @collection self-reference issue. The downside is that only the
// original team creator can manage rosters at the DB layer; promoting a
// non-creator captain to manage the roster requires either an admin action
// or a custom Go-backed route.
func setRostersRules(c *core.Collection) {
	listView := "@request.auth.id != \"\""
	mutate := "@request.auth.isAdmin = true || team.created_by = @request.auth.id"

	c.ListRule = &listView
	c.ViewRule = &listView
	c.CreateRule = &mutate
	c.UpdateRule = &mutate
	c.DeleteRule = &mutate
}
