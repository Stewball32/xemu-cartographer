package pb

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/pocketbase/dbx"
)

// HoldsRole implements authz.Deps: a user_roles row exists for (userID,
// roles.slug). Same two-step lookup as Grant / Revoke (roles by slug, then
// the user_roles row by user + role id) so the answer matches what Revoke
// would delete.
//
// The answer feeds a Deny predicate (denyLastAdmin), so "false" is the open
// answer and fail-closed means telling the rule apart: ok=true with
// holds=false only when a lookup ran and found no row (sql.ErrNoRows —
// nobody holds an unknown slug either); ok=false for a nil adapter, empty
// arguments and every other error, which the rule reads as "refuse".
func (d *PBDeps) HoldsRole(userID, slug string) (holds, ok bool) {
	if d == nil || d.app == nil {
		return false, false
	}
	userID, slug = strings.TrimSpace(userID), strings.TrimSpace(slug)
	if userID == "" || slug == "" {
		return false, false
	}
	role, err := d.app.FindFirstRecordByData("roles", "slug", slug)
	if err != nil || role == nil {
		return false, errors.Is(err, sql.ErrNoRows)
	}
	row, err := d.app.FindFirstRecordByFilter(
		"user_roles",
		"user = {:userID} && role = {:roleID}",
		dbx.Params{"userID": userID, "roleID": role.Id},
	)
	if err != nil || row == nil {
		return false, errors.Is(err, sql.ErrNoRows)
	}
	return true, true
}
