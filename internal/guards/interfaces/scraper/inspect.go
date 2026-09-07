package scraper

import "github.com/Stewball32/xemu-cartographer/internal/scraper/manager"

// Info is one row in the running-scraper list, returned by Lifecycle consumers
// (the /api/admin/scraper GET handler, dashboards, debug routes). Alias of
// manager.Info (moved in step 7 part 3c; type identity preserved).
type Info = manager.Info

// PreviousGameInfo is the just-ended match captured on Live → Ready
// transitions, surfaced by Inspect. Alias of manager.PreviousGameInfo.
type PreviousGameInfo = manager.PreviousGameInfo

// InspectState is the per-runner deep-dive view served by the debug page's
// /api/admin/scraper/{name}/inspect endpoint. Alias of manager.InspectState.
type InspectState = manager.InspectState

// Inspect lets callers enumerate currently-running scrapers and fetch the
// deep-dive cached state for one named runner.
type Inspect interface {
	List() []Info
	Inspect(name string) (InspectState, bool)
}
