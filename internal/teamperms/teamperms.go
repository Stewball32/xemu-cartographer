// Package teamperms answers cross-cutting team-permission questions that
// the M23 invite / join-request / leave / remove routes ask before
// mutating state. Lives outside internal/pocketbase/routes/ because the
// same checks are needed from multiple route groups (teams/,
// team_membership_requests/, rosters/) and the soft-delete cascade in
// internal/pocketbase/hooks/users_soft_delete_pii.go.
//
// Why not a guards/interfaces interface? The scraper + Discord systems
// don't need team-permission checks (their domain is per-instance + per-
// channel state), so adding it to pbiface.Service would couple unrelated
// systems. Direct core.App calls keep the dependency one-directional:
// route handlers + hooks import this package; nothing in this package
// reaches outward.
package teamperms

import (
	"fmt"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// IsOwnerOrManager returns true if userID currently holds an active
// roster row on teamID with is_owner OR is_manager set. The "active"
// gate (`left_at = null`) means a past owner who left is no longer a
// privileged actor — they need a new roster row before they can manage
// the team again.
//
// Walks the rosters collection via the rosters.gamertag.user relation
// chain. Two roster rows for the same user on the same team are both
// counted: this answers "do they have ANY active owner/manager seat?"
// not "do they have this specific one?".
func IsOwnerOrManager(app core.App, userID, teamID string) (bool, error) {
	if userID == "" || teamID == "" {
		return false, nil
	}
	rows, err := app.FindRecordsByFilter(
		"rosters",
		"team = {:teamID} && left_at = null && gamertag.user = {:userID} && (is_owner = true || is_manager = true)",
		"-created",
		1, 0,
		dbx.Params{"teamID": teamID, "userID": userID},
	)
	if err != nil {
		return false, fmt.Errorf("teamperms.IsOwnerOrManager: %w", err)
	}
	return len(rows) > 0, nil
}

// HasActiveMembership returns true if userID currently holds any active
// roster row on teamID (owner, manager, or plain member). Used by the
// invite + join-request handlers to reject "you already belong / they
// already belong" attempts before creating a duplicate request.
func HasActiveMembership(app core.App, userID, teamID string) (bool, error) {
	if userID == "" || teamID == "" {
		return false, nil
	}
	rows, err := app.FindRecordsByFilter(
		"rosters",
		"team = {:teamID} && left_at = null && gamertag.user = {:userID}",
		"-created",
		1, 0,
		dbx.Params{"teamID": teamID, "userID": userID},
	)
	if err != nil {
		return false, fmt.Errorf("teamperms.HasActiveMembership: %w", err)
	}
	return len(rows) > 0, nil
}

// OwnersAndManagers loads the user records for every active owner or
// manager on teamID. Used by the join-request handler to fan out one
// notification per owner/manager. Returned slice is deduplicated by
// user.Id since a single user could hold both an owner roster row and a
// manager roster row (or two owner rows under different gamertags).
func OwnersAndManagers(app core.App, teamID string) ([]*core.Record, error) {
	if teamID == "" {
		return nil, nil
	}
	rosters, err := app.FindRecordsByFilter(
		"rosters",
		"team = {:teamID} && left_at = null && (is_owner = true || is_manager = true)",
		"-created",
		0, 0,
		dbx.Params{"teamID": teamID},
	)
	if err != nil {
		return nil, fmt.Errorf("teamperms.OwnersAndManagers: filter rosters: %w", err)
	}
	if len(rosters) == 0 {
		return nil, nil
	}

	for _, expandErr := range app.ExpandRecords(rosters, []string{"gamertag.user"}, nil) {
		if expandErr != nil {
			return nil, fmt.Errorf("teamperms.OwnersAndManagers: expand: %w", expandErr)
		}
	}

	seen := map[string]bool{}
	out := make([]*core.Record, 0, len(rosters))
	for _, r := range rosters {
		gt := r.ExpandedOne("gamertag")
		if gt == nil {
			continue
		}
		usr := gt.ExpandedOne("user")
		if usr == nil {
			continue
		}
		if seen[usr.Id] {
			continue
		}
		seen[usr.Id] = true
		out = append(out, usr)
	}
	return out, nil
}

// FindPendingRequest returns the single pending team_membership_requests
// row for (teamID, userID) regardless of direction, or nil if none.
// Used by invite + join-request handlers to short-circuit duplicate
// pending state. Treats either-direction pending as a duplicate because
// a user mid-invite shouldn't simultaneously open a join request (and
// vice versa) — the existing pending row is the resolution surface.
func FindPendingRequest(app core.App, teamID, userID string) (*core.Record, error) {
	if teamID == "" || userID == "" {
		return nil, nil
	}
	rows, err := app.FindRecordsByFilter(
		"team_membership_requests",
		"team = {:teamID} && user = {:userID} && status = \"pending\"",
		"-created",
		1, 0,
		dbx.Params{"teamID": teamID, "userID": userID},
	)
	if err != nil {
		return nil, fmt.Errorf("teamperms.FindPendingRequest: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}
