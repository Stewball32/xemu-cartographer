package pb

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/xemu-cartographer/xemu-cartographer/internal/audit"
	"github.com/xemu-cartographer/xemu-cartographer/internal/authz"
)

// ErrForbidden is the generic authorization failure Grant / Revoke / Mint /
// RevokeToken / ListTokens return when Can denies for a reason that has no
// dedicated sentinel (wrong principal kind, missing scope, nil deps). The
// decision reason is wrapped in the message.
var ErrForbidden = errors.New("authz: forbidden")

// ErrRoleNotGrantable is returned by Grant for the "anonymous" slug (400):
// that row is the console door's scope list (PD-1), not a role a user can
// hold — a user_roles row on it would give a signed-in account whatever the
// door hands out, unbounded by any level.
var ErrRoleNotGrantable = errors.New("authz: role cannot be granted to a user")

// depsOr returns d, or an ephemeral adapter over app when d is nil. The
// mutation helpers use it so in-process callers (hooks, the seeder) that run
// before SetDefault still get the same fail-closed data answers; nothing is
// granted that a booted adapter would refuse.
func depsOr(d *PBDeps, app core.App) *PBDeps {
	if d != nil {
		return d
	}
	return newDeps(app, nil, nil)
}

// decisionError maps a denying Decision to the sentinel the callers map to
// HTTP statuses (§6.4): pred:unknown_role → ErrUnknownRole,
// pred:level_ok → ErrLevelTooLow, deny:last_admin → ErrLastAdmin,
// everything else → ErrForbidden (reason attached).
func decisionError(dec authz.Decision) error {
	switch dec.Reason {
	case "pred:unknown_role":
		return authz.ErrUnknownRole
	case "pred:level_ok":
		return authz.ErrLevelTooLow
	case "deny:last_admin":
		return authz.ErrLastAdmin
	}
	return fmt.Errorf("%w (%s)", ErrForbidden, dec.Reason)
}

// Grant creates the user_roles row (userID, slug) and its ActionRoleGrant
// audit row after Can(actor, role.grant, Role(slug, userID)): ErrLevelTooLow
// when the actor's level is below the role's, ErrUnknownRole for an unknown
// slug, ErrRoleNotGrantable for "anonymous" (refused before Can, for every
// actor). Idempotent: an existing row is a silent no-op. granted_by is set
// only for users actors (superusers and internal callers leave it empty).
func Grant(app core.App, d *PBDeps, actor authz.Principal, userID, slug, reason string) error {
	if app == nil {
		return fmt.Errorf("pb.Grant: app is required")
	}
	userID, slug = strings.TrimSpace(userID), strings.TrimSpace(slug)
	if userID == "" {
		return fmt.Errorf("pb.Grant: userID is required")
	}
	if slug == "" {
		return fmt.Errorf("pb.Grant: slug is required")
	}
	if slug == "anonymous" {
		return fmt.Errorf("%w: %q", ErrRoleNotGrantable, slug)
	}
	d = depsOr(d, app)

	if dec := authz.CanWith(d, actor, authz.ActionRoleGrant, authz.Role(slug, userID)); !dec.Allow {
		return decisionError(dec)
	}

	role, err := app.FindFirstRecordByData("roles", "slug", slug)
	if err != nil || role == nil {
		return fmt.Errorf("%w: %q", authz.ErrUnknownRole, slug)
	}

	existing, err := app.FindFirstRecordByFilter(
		"user_roles",
		"user = {:userID} && role = {:roleID}",
		dbx.Params{"userID": userID, "roleID": role.Id},
	)
	if err == nil && existing != nil {
		return nil
	}

	user, err := app.FindRecordById("users", userID)
	if err != nil {
		return fmt.Errorf("pb.Grant: lookup user %s: %w", userID, err)
	}

	col, err := app.FindCollectionByNameOrId("user_roles")
	if err != nil {
		return fmt.Errorf("pb.Grant: lookup user_roles collection: %w", err)
	}
	row := core.NewRecord(col)
	row.Set("user", userID)
	row.Set("role", role.Id)
	if actor.Collection == "users" && actor.UserID != "" {
		row.Set("granted_by", actor.UserID)
	}
	if err := app.Save(row); err != nil {
		return fmt.Errorf("pb.Grant: save user_roles for (user=%s role=%s): %w", userID, slug, err)
	}
	d.InvalidateRoles()

	payload := audit.RoleGrantPayload{
		RoleSlug:    slug,
		Reason:      reason,
		ByMigration: actor.Kind == authz.KindInternal,
		BySuperuser: actor.Kind == authz.KindSuperuser,
		Actor:       auditActorLabel(actor),
	}
	if err := audit.Write(app, actorRecord(app, actor), audit.ActionRoleGrant, user, payload); err != nil {
		return fmt.Errorf("pb.Grant: audit write: %w", err)
	}
	return nil
}

// Revoke deletes the user_roles row (userID, slug) and writes the
// ActionRoleRevoke audit row after Can(actor, role.revoke, Role(slug,
// userID)): ErrLastAdmin when a non-internal actor would remove the last
// admin (the internal cascade is allowed and logged at WARN, §4.4). No-op
// when no row exists.
//
// The decision, the row lookup, the delete and the audit row run in one
// transaction against an uncached adapter over txApp, so two revokes racing
// for the last two admins serialise on the write connection and the second
// one sees the first one's delete: the last_admin deny counts the rows that
// are actually left, never a pre-transaction snapshot.
func Revoke(app core.App, d *PBDeps, actor authz.Principal, userID, slug, reason string) error {
	if app == nil {
		return fmt.Errorf("pb.Revoke: app is required")
	}
	userID, slug = strings.TrimSpace(userID), strings.TrimSpace(slug)
	if userID == "" {
		return fmt.Errorf("pb.Revoke: userID is required")
	}
	if slug == "" {
		return fmt.Errorf("pb.Revoke: slug is required")
	}
	d = depsOr(d, app)

	err := app.RunInTransaction(func(txApp core.App) error {
		return revokeTx(txApp, d.withApp(txApp), actor, userID, slug, reason)
	})
	d.InvalidateRoles()
	return err
}

// revokeTx is the body of Revoke: app is the transaction's app and d an
// uncached adapter over it.
func revokeTx(app core.App, d *PBDeps, actor authz.Principal, userID, slug, reason string) error {
	if dec := authz.CanWith(d, actor, authz.ActionRoleRevoke, authz.Role(slug, userID)); !dec.Allow {
		return decisionError(dec)
	}

	role, err := app.FindFirstRecordByData("roles", "slug", slug)
	if err != nil || role == nil {
		return fmt.Errorf("%w: %q", authz.ErrUnknownRole, slug)
	}

	row, err := app.FindFirstRecordByFilter(
		"user_roles",
		"user = {:userID} && role = {:roleID}",
		dbx.Params{"userID": userID, "roleID": role.Id},
	)
	if err != nil || row == nil {
		return nil
	}

	if actor.Kind == authz.KindInternal && slug == "admin" && d.AdminCount() <= 1 {
		log.Printf("authz: last admin revoked by internal actor=%s user=%s", actor.ID, userID)
		app.Logger().Warn("authz: last admin revoked by internal actor", "actor", actor.ID, "user", userID)
	}

	if err := app.Delete(row); err != nil {
		return fmt.Errorf("pb.Revoke: delete user_roles for (user=%s role=%s): %w", userID, slug, err)
	}

	payload := audit.RoleRevokePayload{
		RoleSlug:    slug,
		Reason:      reason,
		BySuperuser: actor.Kind == authz.KindSuperuser,
		Actor:       auditActorLabel(actor),
	}
	if err := audit.WriteRef(app, actorRecord(app, actor), audit.ActionRoleRevoke, "users", userID, payload); err != nil {
		return fmt.Errorf("pb.Revoke: audit write: %w", err)
	}
	return nil
}

// IsAdmin is the drop-in for roles.IsAdminAuth: superuser or admin-role
// holder; false for nil auth and on any lookup failure.
func IsAdmin(app core.App, d *PBDeps, auth *core.Record) bool {
	if auth == nil {
		return false
	}
	return PrincipalFromAuth(app, depsOr(d, app), auth).IsAdmin()
}

// actorRecord loads the users record for a pb_user actor (the only
// collection the audit_log actor relation accepts); nil otherwise.
func actorRecord(app core.App, actor authz.Principal) *core.Record {
	if app == nil || actor.Collection != "users" || actor.UserID == "" {
		return nil
	}
	rec, err := app.FindRecordById("users", actor.UserID)
	if err != nil {
		return nil
	}
	return rec
}

// auditActorLabel is the payload-side actor for principals the audit_log
// actor relation cannot reference: "discord:<snowflake>" (A.9) and
// "internal:<actor>"; "" for users / superusers (BySuperuser covers those).
func auditActorLabel(actor authz.Principal) string {
	switch actor.Kind {
	case authz.KindDiscord:
		if actor.ID != "" {
			return "discord:" + actor.ID
		}
	case authz.KindInternal:
		if actor.ID != "" {
			return "internal:" + actor.ID
		}
	case authz.KindMachine, authz.KindSpectator, authz.KindDevice:
		if actor.ID != "" {
			return string(actor.Kind) + ":" + actor.ID
		}
	}
	return ""
}
