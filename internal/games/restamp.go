package games

import (
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// RestampEvents re-runs the game-end event stamping for an already
// persisted game: every game_events row of instance with an empty `game` relation
// whose `ts` falls inside [start, end] gets gameID. It exists for the
// step-8 webhook ingest (routes/xc): the daemon's event sink and its
// finished_game webhook are independent deliveries, so a late in-window
// event row can land after PersistFinishedGame already stamped — the route
// schedules one RestampEvents a few seconds after a non-deduped persist.
//
// Safe to call any time: it is the same idempotent stampGameEvents the
// chain runs (only rows with an empty `game` are touched) and the window filter runs
// in Go after the fetch, so the next game's rows on the same instance are
// skipped. A zero start or end leaves that side of the window open; both
// zero stamps every unstamped row of the instance (the chain's own
// behaviour for an artifact without timestamps). Returns the number of
// rows stamped.
func RestampEvents(app core.App, gameID, instance string, start, end time.Time) (int, error) {
	if app == nil {
		return 0, nil
	}
	return stampGameEvents(app, gameID, instance, start, end)
}
