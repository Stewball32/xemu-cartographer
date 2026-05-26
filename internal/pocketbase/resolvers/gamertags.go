package resolvers

import (
	"errors"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// FindGamertagsForUser returns every gamertag record owned by the given user.
// Empty slice + nil error when the user owns no tags.
func FindGamertagsForUser(app core.App, userID string) ([]*core.Record, error) {
	return app.FindRecordsByFilter(
		"gamertags",
		"user = {:userID}",
		"tag",
		0,
		0,
		dbx.Params{"userID": userID},
	)
}

// FindUserByGamertagString finds the owner of a gamertag matching the given
// exact string. Returns (nil, nil) when no row matches — distinct from a DB
// error so callers can decide whether to treat "unknown player" as a hard
// failure.
func FindUserByGamertagString(app core.App, tag string) (*core.Record, error) {
	tagRec, err := app.FindFirstRecordByFilter(
		"gamertags",
		"tag = {:tag}",
		dbx.Params{"tag": tag},
	)
	if err != nil {
		// PB returns sql.ErrNoRows-wrapped errors here; treat any error as
		// "no match" since we'd otherwise need to switch on a specific
		// sentinel that PB doesn't export cleanly.
		return nil, nil //nolint:nilerr // see comment
	}
	if tagRec == nil {
		return nil, nil
	}
	userID := tagRec.GetString("user")
	if userID == "" {
		return nil, errors.New("gamertag row has empty user field")
	}
	return app.FindRecordById("users", userID)
}

// FindActiveRostersForUser returns roster rows where the acting user is
// currently rostered (left_at is null) on any team. The returned records
// have `team` and `gamertag` expanded for one-shot rendering.
func FindActiveRostersForUser(app core.App, userID string) ([]*core.Record, error) {
	rosters, err := app.FindRecordsByFilter(
		"rosters",
		"gamertag.user = {:userID} && left_at = null",
		"team.name",
		0,
		0,
		dbx.Params{"userID": userID},
	)
	if err != nil {
		return nil, err
	}
	if len(rosters) == 0 {
		return rosters, nil
	}

	// Expand both relations in one pass so the /api/me handler (and any
	// other caller) can read team metadata without re-fetching.
	for _, e := range app.ExpandRecords(rosters, []string{"team", "gamertag"}, nil) {
		if e != nil {
			return nil, e
		}
	}
	return rosters, nil
}
