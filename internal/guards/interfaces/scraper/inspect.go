package scraper

import "github.com/xemu-cartographer/xc-scraper/runner"

// Info is one row in the running-scraper list, returned by Lifecycle consumers
// (the /api/admin/scraper GET handler, dashboards, debug routes). Alias of
// runner.Info (moved in step 7 part 3c; type identity preserved).
type Info = runner.Info

// PreviousGameInfo is the just-ended match captured on Live → Ready
// transitions, surfaced by Inspect. Alias of runner.PreviousGameInfo.
type PreviousGameInfo = runner.PreviousGameInfo

// InspectState is the per-runner deep-dive view served by the debug page's
// /api/admin/scraper/{name}/inspect endpoint. Alias of runner.InspectState.
type InspectState = runner.InspectState

// Inspect lets callers enumerate currently-running scrapers and fetch the
// deep-dive cached state for one named runner.
type Inspect interface {
	List() []Info
	Inspect(name string) (InspectState, bool)
}
