package schema

import (
	"fmt"
	"log"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	"github.com/Stewball32/xemu-cartographer/internal/audit"
)

func init() {
	register(registerUsersCollection)
}

// GamertagMaxLen caps users.gamertag — the in-game name, SEPARATE from the
// account `username`. The gamertag is written into BOTH the Halo: CE and Halo 2
// player profiles, so it must fit the SHORTER of the two games' name fields
// (cap = min(CE, H2)):
//
//   - H2: the `profile` name field is a 0xE4-byte (228) UTF-16LE NUL-terminated
//     buffer (halosave/h2profile.go `h2pNameBuf`, cross-checked against 4 real
//     profiles — the name ends at 0xEC, before the per-profile sub-block).
//     228 bytes / 2 (UTF-16) − 1 (NUL) = 113 usable characters.
//   - CE: ~11 chars (24-byte UTF-16 buffer, FORMATS.md §2), but the exact CE
//     name source is still being researched.
//
// Until the CE research lands we default to the H2 limit (the only confirmed
// one). This is the ONE knob to TIGHTEN to CE's smaller value when it lands
// (likely ~11). The frontend mirrors this number — keep them in sync.
const GamertagMaxLen = 113

// registerUsersCollection customizes the built-in "users" auth collection
// that PocketBase creates automatically on first boot.
//
// PocketBase ships the users auth collection with these fields already:
//   - id, password, tokenKey, email, emailVisibility, verified (system/auth)
//   - name (text), avatar (file)
//   - created, updated (timestamps)
//
// Don't redefine those here — only add project-specific fields below.
func registerUsersCollection(app *pocketbase.PocketBase) error {
	// No collectionExists() guard — users always exists, so we reconcile on every boot.
	users, err := requireCollection(app, "users")
	if err != nil {
		return err
	}

	// Guard each field with GetByName so reboots stay idempotent.
	if users.Fields.GetByName("username") == nil {
		users.Fields.Add(&core.TextField{
			Name:        "username",
			Min:         2,
			Max:         34,
			Presentable: true,
			Required:    true,
		})
	}

	// gamertag is the in-game name (SEPARATE from the login `username`, which
	// carries no Halo constraints). It is written into the user's CE + H2 player
	// profiles, so it's length-capped to GamertagMaxLen (min of the two games'
	// name fields). Distinct from default_gamertag (the M07 roster-matching
	// relation) — this is a plain capped text field. Optional: a user sets it
	// before generating profiles; the profile generate hooks read it via the
	// `user` relation, and users_gamertag_regen re-generates the profiles when it
	// changes.
	if users.Fields.GetByName("gamertag") == nil {
		users.Fields.Add(&core.TextField{
			Name: "gamertag",
			Max:  GamertagMaxLen,
		})
	}

	if users.Fields.GetByName("bio") == nil {
		users.Fields.Add(&core.TextField{
			Name: "bio",
			Max:  500,
		})
	}

	if users.Fields.GetByName("location") == nil {
		users.Fields.Add(&core.TextField{
			Name: "location",
			Max:  100,
		})
	}

	// M08d: isAdmin column is no longer added on fresh installs. The
	// migration below drops it on existing installs once every previously-
	// admin user has a user_roles row pointing at the admin role.

	// Soft-delete (M7d). Hard delete loses every FK back through gamertags +
	// rosters + teams.created_by, which would scrub real game history when a
	// user "deletes their account". Instead, mark + tombstone: the
	// users_soft_delete_pii hook blanks email/name/bio/location/avatar when
	// is_deleted flips false→true. Login auth is blocked by the AuthRule
	// below. Reactivation pathway is out of scope; a separate hard-delete
	// path for legal/GDPR may land later.
	if users.Fields.GetByName("is_deleted") == nil {
		users.Fields.Add(&core.BoolField{
			Name: "is_deleted",
		})
	}
	if users.Fields.GetByName("deleted_at") == nil {
		users.Fields.Add(&core.DateField{
			Name: "deleted_at",
		})
	}

	// Ban + timeout (M8f). is_banned is the indefinite-block knob;
	// banned_until is the time-bounded variant. The AuthRule below admits a
	// user when either (a) is_banned is false, or (b) banned_until is in
	// the past — so a stale is_banned=true row whose banned_until elapsed
	// silently unblocks. State transitions are gated + audited by the
	// users_ban_transitions hook (M8f).
	//
	// Both fields are public on the API: admins need them for the
	// /admin/roles/ Assignments tab (see the list/view rules below that
	// admit holders of the admin role), and a banned user themselves can
	// read their own row only — knowing they're banned isn't sensitive.
	if users.Fields.GetByName("is_banned") == nil {
		users.Fields.Add(&core.BoolField{
			Name: "is_banned",
		})
	}
	// M8g: flip the prior Hidden:true on is_banned to visible so the FE
	// /admin/roles/ Assignments tab can render the state badge. Idempotent
	// — only writes if the existing field still has Hidden=true.
	if f, ok := users.Fields.GetByName("is_banned").(*core.BoolField); ok && f != nil && f.Hidden {
		f.Hidden = false
	}
	if users.Fields.GetByName("banned_until") == nil {
		users.Fields.Add(&core.DateField{
			Name: "banned_until",
		})
	}

	// default_gamertag points at the user's "show me as" pick. Nullable —
	// freshly-created users get auto-populated by the
	// users_default_gamertag hook, but the field stays unconstrained at
	// the schema level so admins can clear it without violating Required.
	// Lives on users (not as is_primary on gamertags) so there's a single
	// canonical default per user with no risk of zero or multiple primaries.
	if users.Fields.GetByName("default_gamertag") == nil {
		gamertagsCol, err := app.FindCollectionByNameOrId("gamertags")
		if err != nil {
			return err
		}
		users.Fields.Add(&core.RelationField{
			Name:          "default_gamertag",
			CollectionId:  gamertagsCol.Id,
			MaxSelect:     1,
			CascadeDelete: false,
		})
	}

	// Unique index — idempotent by name
	const idxName = "idx_users_username_unique"
	if users.GetIndex(idxName) == "" {
		users.AddIndex(idxName, true, "username", "")
	}

	users.OAuth2.MappedFields = core.OAuth2KnownFields{
		Name:      "",         // OAuth2 full name  → (unmapped) (full name adds '#0' behind the username)
		AvatarURL: "avatar",   // OAuth2 avatar URL → users.avatar
		Username:  "username", // OAuth2 username   → users.username
		Id:        "",         // OAuth2 id         → (unmapped)
	}

	// Soft-deleted users cannot log in. AuthRule gates every auth method
	// (password, OAuth, OTP) so a tombstoned account stays inaccessible
	// without us touching the individual auth handlers.
	//
	// M8f extends with the ban + timeout gate. Admit when:
	//   - not soft-deleted, AND
	//   - either not banned at all, OR
	//   - banned with a timeout that has already elapsed (passive expiry).
	// An indefinite ban (`is_banned=true` + `banned_until=""`) fails the
	// second branch entirely because the empty-string comparison short-
	// circuits via the explicit `banned_until != ""` test.
	authRule := `is_deleted = false && (is_banned = false || (banned_until != "" && banned_until < @now))`
	users.AuthRule = &authRule

	// List/view/update rules. PB defaults to "self only" (`id = @request.auth.id`)
	// for the built-in users collection; M8g extends to admit admins so the
	// /admin/roles/ Assignments tab can enumerate users and so the ban routes
	// (which use app.Save and would work regardless) round-trip cleanly when
	// the FE re-fetches after a state change. The update rule stays self-only
	// for the field-level concern — users_ban_transitions hook enforces the
	// is_banned / banned_until gate; everything else is the user's own data.
	// Admins still PATCH through the M8g custom routes (which app.Save), so
	// no admin override on update is needed here.
	listView := "id = @request.auth.id || (" + hasAdminRole + ")"
	users.ListRule = &listView
	users.ViewRule = &listView

	if err := app.Save(users); err != nil {
		return err
	}

	// M08b: backfill historical isAdmin=true users into user_roles. Idempotent:
	// skips users that already have a user_roles row pointing at admin. Synthesizes
	// audit_log rows with actor=nil + ByMigration=true so the admin-activity
	// timeline can disambiguate human grants from the M08 cutover.
	return migrateUsersIsAdminToRoles(app)
}

// migrateUsersIsAdminToRoles walks every users row with isAdmin=true and
// ensures a matching (user, admin) row exists in user_roles. Called from the
// tail of registerUsersCollection so the migration runs once per boot and
// recovers from a half-applied state.
//
// Two-phase: (1) backfill any row that still has isAdmin=true into user_roles;
// (2) once every isAdmin=true row has a matching user_roles entry, drop the
// isAdmin field from the users schema. Phase 2 is the 8d watershed — after
// this commit's first boot, the field stops existing and every reader is
// forced through internal/roles.
//
// Drops out cleanly when the `isAdmin` field has already been removed at a
// previous boot. Errors on the audit write are logged but don't abort the
// wider migration. Errors on the user_roles write itself ARE fatal — without
// that row the user loses admin access on the next boot.
//
// Guard for the drop: if any isAdmin=true row failed its user_roles backfill,
// the field stays in place so the next boot retries. This avoids the
// pathological case where a transient failure during migration leaves an
// admin permanently locked out.
func migrateUsersIsAdminToRoles(app *pocketbase.PocketBase) error {
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return fmt.Errorf("M08 migration: lookup users collection: %w", err)
	}
	if users.Fields.GetByName("isAdmin") == nil {
		return nil
	}

	roleRow, err := app.FindFirstRecordByData("roles", "slug", "admin")
	if err != nil || roleRow == nil {
		log.Printf("M08 migration: admin role not seeded; skipping backfill (run will retry next boot)")
		return nil
	}

	urCol, err := app.FindCollectionByNameOrId("user_roles")
	if err != nil {
		return fmt.Errorf("M08 migration: lookup user_roles collection: %w", err)
	}

	rows, err := app.FindRecordsByFilter("users", "isAdmin = true", "", 0, 0)
	if err != nil {
		return fmt.Errorf("M08 migration: load isAdmin users: %w", err)
	}

	pendingBackfill := 0
	for _, u := range rows {
		existing, _ := app.FindFirstRecordByFilter(
			"user_roles",
			"user = {:userID} && role = {:roleID}",
			dbx.Params{"userID": u.Id, "roleID": roleRow.Id},
		)
		if existing != nil {
			continue
		}
		ur := core.NewRecord(urCol)
		ur.Set("user", u.Id)
		ur.Set("role", roleRow.Id)
		if err := app.Save(ur); err != nil {
			pendingBackfill++
			log.Printf("M08 migration: backfill user_roles for user %s failed (will retry next boot): %v", u.Id, err)
			continue
		}
		if err := audit.WriteRef(app, nil, audit.ActionRoleGrant, "users", u.Id, audit.RoleGrantPayload{
			RoleSlug:    "admin",
			ByMigration: true,
		}); err != nil {
			log.Printf("M08 migration: audit row for user %s: %v (will retry next boot)", u.Id, err)
		}
	}

	if pendingBackfill > 0 {
		log.Printf("M08 migration: %d backfill(s) failed; keeping users.isAdmin in place for next boot", pendingBackfill)
		return nil
	}

	users.Fields.RemoveByName("isAdmin")
	if err := app.Save(users); err != nil {
		return fmt.Errorf("M08 migration: drop users.isAdmin field: %w", err)
	}
	log.Printf("M08 migration: dropped users.isAdmin field; roles source-of-truth is user_roles")
	return nil
}
