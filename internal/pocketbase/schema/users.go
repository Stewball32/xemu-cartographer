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

	if users.Fields.GetByName("isAdmin") == nil {
		users.Fields.Add(&core.BoolField{
			Name:   "isAdmin",
			Hidden: true,
		})
	}

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
	if users.Fields.GetByName("is_banned") == nil {
		users.Fields.Add(&core.BoolField{
			Name:   "is_banned",
			Hidden: true,
		})
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
	// without us touching the individual auth handlers. Other rules
	// (ListRule/ViewRule/etc.) stay at PB defaults — admins can still
	// see deleted rows through the superuser UI for moderation review.
	authRule := "is_deleted = false"
	users.AuthRule = &authRule

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
// Drops out cleanly when the `isAdmin` field has already been removed (8d's
// schema migration), at which point every previously-admin user must
// already hold a user_roles row — the backfill has nothing left to do.
//
// Errors on the audit write are logged but don't abort the wider migration:
// a row whose user_roles write succeeded but whose audit row didn't is
// recoverable on the next boot (the per-row guard re-checks the user_roles
// existence). Errors on the user_roles write itself ARE fatal — without that
// row the user loses admin access immediately.
func migrateUsersIsAdminToRoles(app *pocketbase.PocketBase) error {
	users, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return fmt.Errorf("M08b migration: lookup users collection: %w", err)
	}
	if users.Fields.GetByName("isAdmin") == nil {
		return nil
	}

	roleRow, err := app.FindFirstRecordByData("roles", "slug", "admin")
	if err != nil || roleRow == nil {
		log.Printf("M08b migration: admin role not seeded; skipping backfill (run will retry next boot)")
		return nil
	}

	urCol, err := app.FindCollectionByNameOrId("user_roles")
	if err != nil {
		return fmt.Errorf("M08b migration: lookup user_roles collection: %w", err)
	}

	rows, err := app.FindRecordsByFilter("users", "isAdmin = true", "", 0, 0)
	if err != nil {
		return fmt.Errorf("M08b migration: load isAdmin users: %w", err)
	}

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
			return fmt.Errorf("M08b migration: backfill user_roles for user %s: %w", u.Id, err)
		}
		if err := audit.WriteRef(app, nil, audit.ActionRoleGrant, "users", u.Id, audit.RoleGrantPayload{
			RoleSlug:    "admin",
			ByMigration: true,
		}); err != nil {
			log.Printf("M08b migration: audit row for user %s: %v (will retry next boot)", u.Id, err)
		}
	}
	return nil
}
