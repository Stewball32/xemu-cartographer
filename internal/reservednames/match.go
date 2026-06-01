// Package reservednames evaluates the admin-curated `reserved_names`
// collection landed in M22e against a candidate string. Used by the
// gamertag + team create/update hooks to auto-flag matching submissions
// as status="pending" instead of the default "allowed".
//
// Matching is substring-on-lowercase: pattern "ass" matches "Sassy" and
// "BASS". The M22 doc explicitly accepts the false-positive trade-off
// ("basement" contains "ass") because the consequence is a pending flag,
// not a block — admin still gets to make the call.
//
// No caching layer yet. FindAllRecords runs on every create/update of an
// audited collection; at current write volumes (humans typing in
// gamertags + teams, not bulk imports) the per-call cost is fine. Add a
// cache invalidated by OnRecordAfter*("reserved_names") if it becomes a
// hot path.
package reservednames

import (
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// Match scans every reserved-name pattern against candidate and returns the
// first hit. (matched=true, pattern=<the pattern that matched>, err=nil) on
// a hit; (false, "", nil) on a clean miss; (false, "", err) on a DB error.
//
// Candidate is whitespace-trimmed + lowercased before comparison; patterns
// are likewise lowercased at read time so the admin-stored casing is purely
// display.
func Match(app core.App, candidate string) (bool, string, error) {
	candidateLower := strings.ToLower(strings.TrimSpace(candidate))
	if candidateLower == "" {
		return false, "", nil
	}

	rows, err := app.FindAllRecords("reserved_names")
	if err != nil {
		return false, "", err
	}

	for _, row := range rows {
		pattern := row.GetString("pattern")
		patternLower := strings.ToLower(strings.TrimSpace(pattern))
		if patternLower == "" {
			continue
		}
		if strings.Contains(candidateLower, patternLower) {
			return true, pattern, nil
		}
	}
	return false, "", nil
}
