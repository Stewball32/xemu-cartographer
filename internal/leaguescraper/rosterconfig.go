package leaguescraper

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"

	"github.com/xemu-cartographer/xc-scraper/roster"
)

// LoadRosterConfig reads the dummy-filter configuration for one container from the
// database: the container's is_neutral_host flag (containers collection) and
// the global dummy_gamertags allowlist. It is the single source of the filter
// Config, reused by the scraper's server-side filtered broadcast (the
// game_filtered class) AND the HTTP console route's snapshot filter — so both
// surfaces drop exactly the same players.
//
// Best-effort + nil-safe: a nil app or any DB read error yields the zero Config
// (a no-op filter), so a lookup failure never accidentally blanks a roster.
func LoadRosterConfig(app core.App, instance string) roster.Config {
	if app == nil {
		return roster.Config{}
	}
	neutral := false
	if rec, err := app.FindFirstRecordByFilter("containers", "name = {:n}", dbx.Params{"n": instance}); err == nil && rec != nil {
		neutral = rec.GetBool("is_neutral_host")
	}
	var raw []string
	if rows, err := app.FindAllRecords("dummy_gamertags"); err == nil {
		raw = make([]string, 0, len(rows))
		for _, r := range rows {
			raw = append(raw, r.GetString("gamertag"))
		}
	}
	return roster.Config{IsNeutralHost: neutral, DummyGamertags: roster.BuildDummySet(raw)}
}
