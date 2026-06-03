// Package teamlog owns the write path for the team_log collection landed in
// M23c. Callers (route handlers in internal/pocketbase/routes/teams/,
// .../team_membership_requests/, .../rosters/, plus the existing
// teams_status_transitions hook) invoke Write rather than constructing
// team_log records directly, so the payload-JSON shape stays consistent
// across every event type and the per-event structs serve as the canonical
// schema.
//
// # Event + payload convention
//
// Each distinct event lives in its own file named event_<verb>.go (e.g.
// event_member_added.go). The file declares two things:
//
//  1. An Event constant: `const EventMemberAdded Event = "member_added"`.
//  2. A matching payload struct.
//
// Mirrors the internal/audit/ + internal/notifications/ conventions.
package teamlog

import (
	"encoding/json"
	"fmt"

	"github.com/pocketbase/pocketbase/core"
)

// Event is the string-valued discriminator stored in the team_log.event
// column. Named-type so Write can't accept arbitrary strings — callers
// must pass an exported Event constant declared in one of the event_*.go
// files.
type Event string

// Write records a team_log row for an event on team. actor may be nil for
// system-triggered events (soft-delete cascade, future cron jobs).
// subjectUser / subjectGamertag are both optional — set whichever apply
// (member_left has both; team_renamed has neither). payload may be nil
// when the event constant alone carries the meaning.
//
// Returns the error from JSON marshal or app.Save. Callers typically log +
// continue rather than failing the parent operation; losing an activity
// row is degrading, not breaking.
func Write(
	app core.App,
	team *core.Record,
	actor *core.Record,
	e Event,
	subjectUser *core.Record,
	subjectGamertag *core.Record,
	payload any,
) error {
	if team == nil {
		return fmt.Errorf("teamlog.Write: team record is nil")
	}
	if e == "" {
		return fmt.Errorf("teamlog.Write: event is required")
	}

	col, err := app.FindCollectionByNameOrId("team_log")
	if err != nil {
		return fmt.Errorf("teamlog.Write: lookup team_log collection: %w", err)
	}

	rec := core.NewRecord(col)
	rec.Set("team", team.Id)
	rec.Set("event", string(e))
	if actor != nil {
		rec.Set("actor", actor.Id)
	}
	if subjectUser != nil {
		rec.Set("subject_user", subjectUser.Id)
	}
	if subjectGamertag != nil {
		rec.Set("subject_gamertag", subjectGamertag.Id)
	}

	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("teamlog.Write: marshal payload for event %q: %w", e, err)
		}
		rec.Set("payload_json", string(b))
	}

	if err := app.Save(rec); err != nil {
		return fmt.Errorf("teamlog.Write: save row for event %q: %w", e, err)
	}
	return nil
}
