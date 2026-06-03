// Package notifications owns the write path for the notifications collection
// landed in M23a. Callers (typically route handlers in
// internal/pocketbase/routes/teams/, .../team_membership_requests/, and
// .../rosters/) invoke Notify rather than constructing notifications records
// directly, so the payload-JSON shape stays consistent across every event
// type and the per-type structs serve as the canonical schema.
//
// # Type + payload convention
//
// Each notification type lives in its own file named notification_<event>.go
// (e.g. notification_team_invite.go). The file declares two things:
//
//  1. A Type constant: `const NotifTypeTeamInvite Type = "team_invite"`.
//  2. A matching payload struct: `type TeamInvitePayload struct { ... }`.
//
// Callers pair them: notifications.Notify(app, recipient, NotifTypeTeamInvite,
// TeamInvitePayload{...}). The one-file-per-type layout matches the
// internal/audit/ convention shipped in M22a.
package notifications

import (
	"encoding/json"
	"fmt"

	"github.com/pocketbase/pocketbase/core"
)

// Type is the string-valued discriminator stored in the notifications.type
// column. Defined as a named type so Notify can't accept arbitrary strings —
// callers must pass an exported Type constant declared in one of the
// notification_*.go files.
type Type string

// Notify writes a notifications row addressed to user with the given type +
// payload. The recipient must be a non-nil users record; pass payload=nil
// for notifications whose existence is the whole story (the type alone tells
// the recipient what to do).
//
// Returns the error from the JSON marshal or app.Save. Route handlers
// typically log + continue on error rather than rolling back the parent
// operation — losing a notification is degrading, not breaking.
func Notify(app core.App, user *core.Record, t Type, payload any) error {
	if user == nil {
		return fmt.Errorf("notifications.Notify: user record is nil")
	}
	if t == "" {
		return fmt.Errorf("notifications.Notify: type is required")
	}

	notifsCol, err := app.FindCollectionByNameOrId("notifications")
	if err != nil {
		return fmt.Errorf("notifications.Notify: lookup notifications collection: %w", err)
	}

	rec := core.NewRecord(notifsCol)
	rec.Set("user", user.Id)
	rec.Set("type", string(t))
	rec.Set("read", false)

	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("notifications.Notify: marshal payload for type %q: %w", t, err)
		}
		rec.Set("payload_json", string(b))
	}

	if err := app.Save(rec); err != nil {
		return fmt.Errorf("notifications.Notify: save row for type %q: %w", t, err)
	}
	return nil
}
