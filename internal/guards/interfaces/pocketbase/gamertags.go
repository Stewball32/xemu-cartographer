package pocketbase

import "github.com/pocketbase/pocketbase/core"

// Gamertags abstracts gamertag + team-membership lookups so non-PB systems
// (scraper, Discord) can resolve player identity without taking a hard
// import dependency on the PB packages.
//
// Method scope is intentionally narrow — the /api/me handler reads the same
// data directly via core.App, since it already lives inside the PB system.
// These methods exist so the scraper can later correlate captured player
// names against known users (M9+).
type Gamertags interface {
	// FindGamertagsForUser returns every gamertag owned by the given user
	// (including blocked rows — callers filter if they care).
	FindGamertagsForUser(userID string) ([]*core.Record, error)

	// FindUserByGamertagString looks up the owner of a gamertag by its
	// exact string. Returns (nil, nil) if no tag matches. Case-sensitive.
	FindUserByGamertagString(tag string) (*core.Record, error)

	// FindActiveRostersForUser returns roster rows where the user is
	// currently active (left_at is null) on any team, with `team` and
	// `gamertag` pre-expanded so callers can render team metadata without
	// extra round-trips.
	FindActiveRostersForUser(userID string) ([]*core.Record, error)
}
