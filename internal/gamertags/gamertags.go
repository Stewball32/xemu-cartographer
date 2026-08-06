// Package gamertags holds the small read helper M09 needs to test a user's
// gamertags against live container rosters. It exists as a leaf package
// (core.App + dbx only) so the three consumers that need it — the WS room
// guard, the kiosk/VNC proxy, and the play roster resolver — share one
// query + sanitization path without importing each other.
package gamertags

import (
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// SanitizedForUser returns the lowercased+trimmed gamertag strings the user
// owns, excluding blocked rows (a blocked tag must not grant match access).
// Reads the gamertags.sanitized column maintained by the gamertags_sanitize
// hook, falling back to sanitizing the raw tag if a row predates the hook.
// Returns (nil, nil) for an empty userID or a user with no usable tags.
func SanitizedForUser(app core.App, userID string) ([]string, error) {
	if app == nil || userID == "" {
		return nil, nil
	}
	rows, err := app.FindRecordsByFilter(
		"gamertags",
		"user = {:userID} && status != 'blocked'",
		"", 0, 0,
		dbx.Params{"userID": userID},
	)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		s := r.GetString("sanitized")
		if s == "" {
			s = strings.ToLower(strings.TrimSpace(r.GetString("tag")))
		}
		if s != "" {
			out = append(out, s)
		}
	}
	return out, nil
}
