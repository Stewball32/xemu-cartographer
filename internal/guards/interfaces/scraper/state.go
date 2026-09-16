package scraper

import "github.com/xemu-cartographer/xc-scraper/runner"

// InstanceState is the per-instance scraper view surfaced to the container
// detail page (game title + Xbox console name + running flag). Alias of
// runner.InstanceState (moved in step 7 part 3c).
type InstanceState = runner.InstanceState

// State lets callers fetch a single runner's surfaced state by name.
type State interface {
	InstanceState(name string) (InstanceState, bool)
}
